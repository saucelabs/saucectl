package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

var targetDir = "/home/testrunner/tests"

// Handler represents the client to handle Docker tasks
type Handler struct {
	client *client.Client
}

// Create generates a docker client
func Create() (*Handler, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	handler := &Handler{
		client: cl,
	}

	return handler, nil
}

// ValidateDependency checks if external dependencies are installed
func (handler Handler) ValidateDependency() error {
	_, err := handler.client.ContainerList(context.Background(), types.ContainerListOptions{})
	return err
}

// HasBaseImage checks if base image is installed
func (handler Handler) HasBaseImage(ctx context.Context, baseImage string) (bool, error) {
	listFilters := filters.NewArgs(
		filters.Arg("reference", baseImage))
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
func (handler Handler) PullBaseImage(ctx context.Context, baseImage string) error {
	options := types.ImagePullOptions{}
	responseBody, err := handler.client.ImagePull(ctx, baseImage, options)

	if err != nil {
		return err
	}

	defer responseBody.Close()
	return nil
}

// StartContainer starts the Docker testrunner container
func (handler Handler) StartContainer(ctx context.Context, baseImage string) (*container.ContainerCreateCreatedBody, error) {
	c, err := handler.client.ContainerCreate(ctx, &container.Config{
		Image: baseImage,
		Env:   []string{"SAUCE_USERNAME", "SAUCE_ACCESS_KEY"},
		Tty:   true,
	}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	if err := handler.client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	// We need to check the tty _before_ we do the ContainerExecCreate, because
	// otherwise if we error out we will leak execIDs on the server (and
	// there's no easy way to clean those up). But also in order to make "not
	// exist" errors take precedence we do a dummy inspect first.
	if _, err := handler.client.ContainerInspect(ctx, c.ID); err != nil {
		return nil, err
	}

	return &c, nil
}

// CopyTestFilesToContainer copies files from the config into the container
func (handler Handler) CopyTestFilesToContainer(ctx context.Context, srcContainerID string, files []string) error {
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

			srcFile := filepath.Join(pwd, file)
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
			handler.client.CopyToContainer(ctx, srcContainerID, targetDir, &buf, types.CopyToContainerOptions{})
		}
	}
	return nil
}

// ExecuteTest runs the test in the Docker container
func (handler Handler) ExecuteTest(ctx context.Context, srcContainerID string) (int, error) {
	execConfig := &types.ExecConfig{
		Cmd: []string{"cd", "/home/testrunner", "&&", "npm", "test"},
	}

	createResp, err := handler.client.ContainerExecCreate(ctx, srcContainerID, *execConfig)
	if err != nil {
		return 1, err
	}

	execStartCheck := types.ExecStartCheck{}
	attachResp, err := handler.client.ContainerExecAttach(ctx, createResp.ID, execStartCheck)
	if err != nil {
		return 1, err
	}

	defer attachResp.Close()

	inspectResp, err := handler.client.ContainerExecInspect(ctx, createResp.ID)
	if err != nil {
		return 1, err
	}

	return inspectResp.ExitCode, nil
}
