package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/system"

	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/streams"
	"github.com/saucelabs/saucectl/cli/utils"
)

var (
	containerStopTimeout   = time.Duration(10) * time.Second
	containerRemoveOptions = types.ContainerRemoveOptions{
		Force:         true,
		RemoveLinks:   false,
		RemoveVolumes: false,
	}
)

// ClientInterface describes the interface used to handle docker commands
type ClientInterface interface {
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
	ContainerExecAttach(ctx context.Context, execID string, config types.ExecConfig) (types.HijackedResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (types.ContainerExecInspect, error)
	ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
}

// Handler represents the client to handle Docker tasks
type Handler struct {
	client ClientInterface
}

// Create generates a docker client
func Create() (*Handler, error) {
	cl, err := client.NewEnvClient()
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

// PullBaseImage pulls an image from Docker
func (handler *Handler) PullBaseImage(ctx context.Context, baseImage string) error {
	options := types.ImagePullOptions{}
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
func (handler *Handler) StartContainer(ctx context.Context, c config.JobConfiguration) (*container.ContainerCreateCreatedBody, error) {
	container, err := handler.client.ContainerCreate(ctx, &container.Config{
		Image: c.Image.Base,
		Env: []string{
			"SAUCE_USERNAME=" + os.Getenv("SAUCE_USERNAME"),
			"SAUCE_ACCESS_KEY=" + os.Getenv("SAUCE_ACCESS_KEY")},
	}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	if err := handler.client.ContainerStart(ctx, container.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
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

// CopyTestFilesToContainer copies files from the config into the container
func (handler *Handler) CopyTestFilesToContainer(ctx context.Context, srcContainerID string, files []string, targetDir string) error {
	for _, pattern := range files {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range matches {
			pwd, err := os.Getwd()
			if err != nil {
				continue
			}

			srcFile := file
			if !path.IsAbs(srcFile) {
				srcFile = filepath.Join(pwd, file)
			}
			file, err := os.Stat(srcFile)
			if err != nil {
				continue
			}

			header, err := tar.FileInfoHeader(file, file.Name())
			if err != nil {
				continue
			}

			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)
			header.Name = file.Name()
			if err := tw.WriteHeader(header); err != nil {
				continue
			}

			f, err := os.Open(srcFile)
			if err != nil {
				continue
			}

			if _, err := io.Copy(tw, f); err != nil {
				continue
			}

			f.Close()

			// use &buf as argument for content in CopyToContainer
			if err := handler.client.CopyToContainer(ctx, srcContainerID, targetDir, &buf, types.CopyToContainerOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
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
	if err == nil && srcStat.Mode&os.ModeSymlink != 0 {
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
	if len(srcInfo.RebaseName) != 0 {
		_, srcBase := archive.SplitPathDirEntry(srcInfo.Path)
		preArchive = archive.RebaseArchiveEntries(content, srcBase, srcInfo.RebaseName)
	}
	return archive.CopyTo(preArchive, srcInfo, dstPath)
}

// ExecuteTest runs the test in the Docker container
func (handler *Handler) ExecuteTest(ctx context.Context, srcContainerID string) (*types.IDResponse, *types.HijackedResponse, error) {
	execConfig := types.ExecConfig{
		Cmd:          []string{"npm", "test"},
		AttachStdout: true,
		AttachStderr: true,
	}

	createResp, err := handler.client.ContainerExecCreate(ctx, srcContainerID, execConfig)
	if err != nil {
		return nil, nil, err
	}

	execStartCheck := types.ExecConfig{
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
