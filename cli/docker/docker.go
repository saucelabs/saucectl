package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

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
func (handler Handler) HasBaseImage(baseImage string) (bool, error) {
	listFilters := filters.NewArgs(
		filters.Arg("reference", baseImage))
	options := types.ImageListOptions{
		All:     true,
		Filters: listFilters,
	}
	ctx := context.Background()
	images, err := handler.client.ImageList(ctx, options)
	if err != nil {
		return false, err
	}

	return len(images) > 0, nil
}

// PullBaseImage pulls an image from Docker
func (handler Handler) PullBaseImage(baseImage string) error {
	ctx := context.Background()
	options := types.ImagePullOptions{}
	responseBody, err := handler.client.ImagePull(ctx, baseImage, options)

	if err != nil {
		return err
	}

	defer responseBody.Close()
	return nil
}

// StartContainer starts the Docker testrunner container
func (handler Handler) StartContainer(baseImage string) error {
	ctx := context.Background()
	resp, err := handler.client.ContainerCreate(ctx, &container.Config{
		Image: baseImage,
		Env:   []string{"SAUCE_USERNAME", "SAUCE_ACCESS_KEY"},
		Tty:   true,
	}, nil, nil, "")
	if err != nil {
		return err
	}

	if err := handler.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	// We need to check the tty _before_ we do the ContainerExecCreate, because
	// otherwise if we error out we will leak execIDs on the server (and
	// there's no easy way to clean those up). But also in order to make "not
	// exist" errors take precedence we do a dummy inspect first.
	if _, err := handler.client.ContainerInspect(ctx, resp.ID); err != nil {
		return err
	}

	// // execConfig := &types.ExecConfig{
	// // 	Cmd: []string{"cd", "/home/runner", "&&", "npm", "test"},
	// // }

	// statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	// select {
	// case err := <-errCh:
	// 	if err != nil {
	// 		return err
	// 	}
	// case <-statusCh:
	// }

	// out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	// if err != nil {
	// 	return err
	// }

	// stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	return nil
}
