package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/go-connections/nat"
	"github.com/phayes/freeport"
	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/cli/streams"
	"github.com/saucelabs/saucectl/cli/utils"
	"github.com/saucelabs/saucectl/internal/playwright"
)

var (
	containerStopTimeout   = time.Duration(10) * time.Second
	containerRemoveOptions = types.ContainerRemoveOptions{
		Force:         true,
		RemoveLinks:   false,
		RemoveVolumes: false,
	}
)

// CommonAPIClient is the interface for interacting with containers.
type CommonAPIClient interface {
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error)
	ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	CopyToContainer(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error
	ContainerStatPath(ctx context.Context, containerID, path string) (types.ContainerPathStat, error)
	CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, types.ContainerPathStat, error)
	ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (types.ContainerExecInspect, error)
	ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
}

// Handler represents the client to handle Docker tasks
type Handler struct {
	client CommonAPIClient
}

// CreateMock allows to get a handler with a custom interface
func CreateMock(client CommonAPIClient) *Handler {
	return &Handler{client}
}

// Create generates a docker client
func Create() (*Handler, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	handler := Handler{
		client: cl,
	}

	return &handler, nil
}

// ValidateDependency checks if external dependencies are installed
func (handler *Handler) ValidateDependency() error {
	_, err := handler.client.ContainerList(context.Background(), types.ContainerListOptions{})
	return err
}

// HasBaseImage checks if base image is installed
func (handler *Handler) HasBaseImage(ctx context.Context, baseImage string) (bool, error) {
	listFilters := filters.NewArgs()
	listFilters.Add("reference", baseImage)
	options := types.ImageListOptions{
		All:     true,
		Filters: listFilters,
	}

	images, err := handler.client.ImageList(ctx, options)
	if err != nil {
		return false, err
	}

	return len(images) > 0, nil
}

// GetImageFlavor returns a string that contains the image name and tag defined by the project.
func (handler *Handler) GetImageFlavor(img config.Image) string {
	// TODO - move this to ImageDefinition
	tag := "latest"
	if img.Tag != "" {
		tag = img.Tag
	}
	return fmt.Sprintf("%s:%s", img.Name, tag)
}

// RegistryUsernameEnvKey represents the username environment variable for authenticating against a docker registry.
const RegistryUsernameEnvKey = "REGISTRY_USERNAME"

// RegistryPasswordEnvKey represents the password environment variable for authenticating against a docker registry.
const RegistryPasswordEnvKey = "REGISTRY_PASSWORD"

// NewImagePullOptions returns a new types.ImagePullOptions object. Credentials are also configured, if available
// via environment variables (see RegistryUsernameEnvKey and RegistryPasswordEnvKey).
func NewImagePullOptions() (types.ImagePullOptions, error) {
	options := types.ImagePullOptions{}
	registryUser, hasRegistryUser := os.LookupEnv(RegistryUsernameEnvKey)
	registryPwd, hasRegistryPwd := os.LookupEnv(RegistryPasswordEnvKey)
	// setup auth https://github.com/moby/moby/blob/master/api/types/client.go#L255
	if hasRegistryUser && hasRegistryPwd {
		log.Debug().Msg("Using registry environment variables credentials")
		authConfig := types.AuthConfig{
			Username: registryUser,
			Password: registryPwd,
		}
		authJSON, err := json.Marshal(authConfig)
		if err != nil {
			return options, err
		}
		authStr := base64.URLEncoding.EncodeToString(authJSON)
		options.RegistryAuth = authStr
	}
	return options, nil
}

// PullBaseImage pulls an image from Docker
func (handler *Handler) PullBaseImage(ctx context.Context, img config.Image) error {
	options, err := NewImagePullOptions()
	if err != nil {
		return err
	}
	baseImage := handler.GetImageFlavor(img)
	responseBody, err := handler.client.ImagePull(ctx, baseImage, options)
	if err != nil {
		return err
	}

	defer responseBody.Close()

	/**
	 * ToDo(Christian): handle stdout
	 */
	out := streams.NewOut(ioutil.Discard)
	if err := jsonmessage.DisplayJSONMessagesToStream(responseBody, out, nil); err != nil {
		return err
	}
	return nil
}

// StartContainer starts the Docker testrunner container
func (handler *Handler) StartContainer(ctx context.Context, c playwright.Project) (*container.ContainerCreateCreatedBody, error) {
	var (
		ports        map[nat.Port]struct{}
		portBindings map[nat.Port][]nat.PortBinding
	)

	port, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}

	// binding port for accessing Chrome DevTools from outside
	// of the container
	ports, portBindings, err = nat.ParsePortSpecs(
		[]string{fmt.Sprintf("%d:9222", port)},
	)
	if err != nil {
		return nil, err
	}

	files := []string{
		//c.Playwright.ConfigFile,
		c.Playwright.ProjectPath,
	}

	//if c.Playwright.EnvFile != "" {
	//	files = append(files, c.Playwright.EnvFile)
	//}

	img := handler.GetImageFlavor(c.Docker.Image)
	pDir, err := handler.ProjectDir(ctx, img)
	if err != nil {
		return nil, err
	}

	var m []mount.Mount
	if c.Docker.FileTransfer == config.DockerFileMount {
		m, err = createMounts(files, pDir)
		if err != nil {
			return nil, err
		}
	}

	username := ""
	accessKey := ""
	if creds := credentials.Get(); creds != nil {
		username = creds.Username
		accessKey = creds.AccessKey
		log.Info().Msgf("Using credentials set by %s", creds.Source)
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Mounts:       m,
	}
	networkConfig := &network.NetworkingConfig{}
	containerConfig := &container.Config{
		Image:        img,
		ExposedPorts: ports,
		Env: []string{
			fmt.Sprintf("SAUCE_USERNAME=%s", username),
			fmt.Sprintf("SAUCE_ACCESS_KEY=%s", accessKey),
		},
	}

	container, err := handler.client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, "")
	if err != nil {
		return nil, err
	}

	log.Info().Str("img", img).Str("id", container.ID[:12]).Msg("Starting container")
	if err := handler.client.ContainerStart(ctx, container.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	if c.Docker.FileTransfer == config.DockerFileCopy {
		if err := copyTestFiles(ctx, handler, container.ID, files, pDir); err != nil {
			return nil, err
		}
	}

	// We need to check the tty _before_ we do the ContainerExecCreate, because
	// otherwise if we error out we will leak execIDs on the server (and
	// there's no easy way to clean those up). But also in order to make "not
	// exist" errors take precedence we do a dummy inspect first.
	if _, err := handler.client.ContainerInspect(ctx, container.ID); err != nil {
		return nil, err
	}

	return &container, nil
}

// copyTestFiles copies the files within the container.
func copyTestFiles(ctx context.Context, handler *Handler, containerID string, files []string, pDir string) error {
	for _, file := range files {
		log.Info().Str("from", file).Str("to", pDir).Msg("File copied")
		if err := handler.CopyToContainer(ctx, containerID, file, pDir); err != nil {
			return err
		}
	}
	return nil
}

// createMounts returns a list of mount bindings, binding files to target, such that {source}:{target}/{source}.
func createMounts(files []string, target string) ([]mount.Mount, error) {
	mm := make([]mount.Mount, len(files))
	for i, f := range files {
		absF, err := filepath.Abs(f)
		if err != nil {
			return mm, err
		}

		dest := path.Join(target, filepath.Base(f))

		mm[i] = mount.Mount{
			Type:          mount.TypeBind,
			Source:        absF,
			Target:        dest,
			ReadOnly:      false,
			Consistency:   mount.ConsistencyDefault,
			BindOptions:   nil,
			VolumeOptions: nil,
			TmpfsOptions:  nil,
		}

		log.Info().Str("from", f).Str("to", dest).Msg("File mounted")
	}

	return mm, nil
}

// CopyFilesToContainer copies the given files into the container.
func (handler *Handler) CopyFilesToContainer(ctx context.Context, srcContainerID string, files []string, targetDir string) error {
	for _, fpath := range files {
		if err := handler.CopyToContainer(ctx, srcContainerID, fpath, targetDir); err != nil {
			return err
		}
	}
	return nil
}

// CopyToContainer copies the given file to the container.
func (handler *Handler) CopyToContainer(ctx context.Context, containerID string, srcFile string, targetDir string) error {
	srcInfo, err := archive.CopyInfoSourcePath(srcFile, true)
	if err != nil {
		return err
	}

	srcArchive, err := archive.TarResource(srcInfo)
	if err != nil {
		return err
	}
	defer srcArchive.Close()

	dstInfo := archive.CopyInfo{IsDir: srcInfo.IsDir, Path: filepath.Join(targetDir, filepath.Base(srcInfo.Path))}
	resolvedDstPath, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, dstInfo)
	if err != nil {
		return err
	}
	defer preparedArchive.Close()

	return handler.client.CopyToContainer(ctx, containerID, resolvedDstPath, preparedArchive, types.CopyToContainerOptions{})
}

// CopyFromContainer downloads a file from the testrunner container
func (handler *Handler) CopyFromContainer(ctx context.Context, srcContainerID string, srcPath string, dstPath string) error {
	if err := utils.ValidateOutputPath(dstPath); err != nil {
		return err
	}

	// if client requests to follow symbol link, then must decide target file to be copied
	var rebaseName string
	srcStat, err := handler.client.ContainerStatPath(ctx, srcContainerID, srcPath)
	if err != nil {
		return err
	}

	// If the destination is a symbolic link, we should follow it.
	if srcStat.Mode&os.ModeSymlink != 0 {
		linkTarget := srcStat.LinkTarget
		if !system.IsAbs(linkTarget) {
			// Join with the parent directory.
			srcParent, _ := archive.SplitPathDirEntry(srcPath)
			linkTarget = filepath.Join(srcParent, linkTarget)
		}

		linkTarget, rebaseName = archive.GetRebaseName(srcPath, linkTarget)
		srcPath = linkTarget
	}

	content, stat, err := handler.client.CopyFromContainer(ctx, srcContainerID, srcPath)
	if err != nil {
		return err
	}
	defer content.Close()

	srcInfo := archive.CopyInfo{
		Path:       srcPath,
		Exists:     true,
		IsDir:      stat.Mode.IsDir(),
		RebaseName: rebaseName,
	}

	preArchive := content
	if srcInfo.RebaseName != "" {
		_, srcBase := archive.SplitPathDirEntry(srcInfo.Path)
		preArchive = archive.RebaseArchiveEntries(content, srcBase, srcInfo.RebaseName)
	}
	return archive.CopyTo(preArchive, srcInfo, dstPath)
}

// Execute runs the test in the Docker container and attaches to its stdout
func (handler *Handler) Execute(ctx context.Context, srcContainerID string, cmd []string, env map[string]string) (*types.IDResponse, *types.HijackedResponse, error) {
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Set env vars for a particular suite
	if len(env) > 0 {
		envVars := []string{}
		for k, v := range env {
			envVars = append(envVars, k+"="+v)
		}
		execConfig.Env = envVars
	}

	createResp, err := handler.client.ContainerExecCreate(ctx, srcContainerID, execConfig)
	if err != nil {
		return nil, nil, err
	}

	execStartCheck := types.ExecStartCheck{
		Tty: false,
	}
	resp, err := handler.client.ContainerExecAttach(ctx, createResp.ID, execStartCheck)
	return &createResp, &resp, err
}

// ExecuteInspect checks exit code of test
func (handler *Handler) ExecuteInspect(ctx context.Context, srcContainerID string) (int, error) {
	inspectResp, err := handler.client.ContainerExecInspect(ctx, srcContainerID)
	if err != nil {
		return 1, err
	}

	return inspectResp.ExitCode, nil
}

// ContainerStop stops a running container
func (handler *Handler) ContainerStop(ctx context.Context, srcContainerID string) error {
	return handler.client.ContainerStop(ctx, srcContainerID, &containerStopTimeout)
}

// ContainerRemove removes testrunner container
func (handler *Handler) ContainerRemove(ctx context.Context, srcContainerID string) error {
	return handler.client.ContainerRemove(ctx, srcContainerID, containerRemoveOptions)
}

// ProjectDir returns the project directory as is configured for the given image.
func (handler *Handler) ProjectDir(ctx context.Context, imageID string) (string, error) {
	ii, _, err := handler.client.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		return "", err
	}

	// The image can tell us via a label where saucectl should mount the project files.
	// We default to the working dir of the container as the default mounting target.
	p := ii.Config.WorkingDir
	if v := ii.Config.Labels["com.saucelabs.project-dir"]; v != "" {
		p = v
	}

	return p, nil
}

// ContainerInspect returns the container information.
func (handler *Handler) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return handler.client.ContainerInspect(ctx, containerID)
}

// IsErrNotFound returns true if the error is a NotFound error, which is returned
// by the API when some object is not found.
func (handler *Handler) IsErrNotFound(err error) bool {
	return client.IsErrNotFound(err)
}
