package mocks

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

// FakeClient Docker mock
type FakeClient struct {
	ContainerListSuccess        bool
	ImageListSuccess            bool
	ImagePullSuccess            bool
	ContainerCreateSuccess      bool
	ContainerStartSuccess       bool
	ContainerInspectSuccess     bool
	CopyToContainerFn           func(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error
	ContainerStatPathSuccess    bool
	CopyFromContainerSuccess    bool
	ContainerExecCreateSuccess  bool
	ContainerExecAttachSuccess  bool
	ContainerExecInspectSuccess bool
	ContainerStopSuccess        bool
	ContainerRemoveSuccess      bool
}

type fakeReadWriteCloser struct{}

func (rwc fakeReadWriteCloser) Read(p []byte) (n int, err error)  { return 1, nil }
func (rwc fakeReadWriteCloser) Close() error                      { return nil }
func (rwc fakeReadWriteCloser) Write(p []byte) (n int, err error) { return 0, nil }

// ContainerList mock function
func (fc *FakeClient) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	if fc.ContainerListSuccess {
		return []types.Container{}, nil
	}
	return nil, errors.New("ContainerListFailure")
}

// ImageList mock function
func (fc *FakeClient) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	if fc.ImageListSuccess {
		return []types.ImageSummary{
			{}, {},
		}, nil
	}

	return nil, errors.New("ImageListFailure")
}

// ImagePull mock function
func (fc *FakeClient) ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error) {
	if fc.ImagePullSuccess {
		return fakeReadWriteCloser{}, nil
	}
	return nil, errors.New("ImagePullFailure")
}

// ContainerCreate mock function
func (fc *FakeClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error) {
	if fc.ContainerCreateSuccess {
		return container.ContainerCreateCreatedBody{}, nil
	}
	return container.ContainerCreateCreatedBody{}, errors.New("ContainerCreateFailure")
}

// ContainerStart mock function
func (fc *FakeClient) ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error {
	if fc.ContainerStartSuccess {
		return nil
	}
	return errors.New("ContainerStartFailure")
}

// ContainerInspect mock function
func (fc *FakeClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	if fc.ContainerInspectSuccess {
		return types.ContainerJSON{}, nil
	}
	return types.ContainerJSON{}, errors.New("ContainerInspectFailure")
}

// CopyToContainer mock function
func (fc *FakeClient) CopyToContainer(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error {
	return fc.CopyToContainerFn(ctx, container, path, content, options)
}

// ContainerStatPath mock function
func (fc *FakeClient) ContainerStatPath(ctx context.Context, containerID, path string) (types.ContainerPathStat, error) {
	if fc.ContainerStatPathSuccess {
		return types.ContainerPathStat{}, nil
	}
	return types.ContainerPathStat{}, errors.New("ContainerStatPathFailure")
}

// CopyFromContainer mock function
func (fc *FakeClient) CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
	if fc.CopyFromContainerSuccess {
		return fakeReadWriteCloser{}, types.ContainerPathStat{}, nil
	}
	return nil, types.ContainerPathStat{}, errors.New("CopyFromContainerFailure")
}

// ContainerExecCreate mock function
func (fc *FakeClient) ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error) {
	if fc.ContainerExecCreateSuccess {
		return types.IDResponse{}, nil
	}
	return types.IDResponse{}, errors.New("ContainerExecCreateFailure")
}

// ContainerExecAttach mock function
func (fc *FakeClient) ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error) {
	if fc.ContainerExecAttachSuccess {
		return types.HijackedResponse{}, nil
	}
	return types.HijackedResponse{}, errors.New("ContainerExecAttachFailure")
}

// ContainerExecInspect mock function
func (fc *FakeClient) ContainerExecInspect(ctx context.Context, execID string) (types.ContainerExecInspect, error) {
	if fc.ContainerExecInspectSuccess {
		return types.ContainerExecInspect{ExitCode: 0}, nil
	}
	return types.ContainerExecInspect{}, errors.New("ContainerExecInspectFailure")
}

// ContainerStop mock function
func (fc *FakeClient) ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error {
	if fc.ContainerStopSuccess {
		return nil
	}
	return errors.New("ContainerStopFailure")
}

// ContainerRemove mock function
func (fc *FakeClient) ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error {
	if fc.ContainerRemoveSuccess {
		return nil
	}
	return errors.New("ContainerRemoveFailure")
}
