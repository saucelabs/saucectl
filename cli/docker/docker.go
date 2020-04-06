package docker

import (
	"context"

	"github.com/docker/docker/api/types"
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

	handler := Handler{
		client: cl,
	}

	return &handler, nil
}

// ValidateDependency checks if external dependencies are installed
func (handler Handler) ValidateDependency() error {
	_, err := handler.client.ContainerList(context.Background(), types.ContainerListOptions{})
	return err
}
