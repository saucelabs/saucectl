package docker

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/cli/config"
	"testing"

	"github.com/saucelabs/saucectl/cli/mocks"
	"gotest.tools/v3/fs"
)

func TestLocalRunnerSetup(t *testing.T) {
	type PassFailCase struct {
		Name          string
		Client        *Handler
		ExpectedError error
	}

	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yaml", "foo: bar", fs.WithMode(0755)))

	cases := []PassFailCase{
		{"docker is not installed", CreateMock(&mocks.FakeClient{}), errors.New("docker is not installed")},
		{"Pulling fails", CreateMock(&mocks.FakeClient{
			ContainerListSuccess: true,
		}), errors.New("ImagePullFailure")},
		{"Creating container fails", CreateMock(&mocks.FakeClient{
			ContainerListSuccess: true,
			ImagePullSuccess:     true,
		}), errors.New("ContainerCreateFailure")},
		{"Copy from container fails", CreateMock(&mocks.FakeClient{
			ContainerListSuccess:    true,
			ImagePullSuccess:        true,
			ContainerStartSuccess:   true,
			ContainerCreateSuccess:  true,
			ContainerInspectSuccess: true,
		}), errors.New("CopyFromContainerFailure")},
		// {"Can not find container config", docker.CreateMock(&mocks.FakeClient{
		// 	ContainerListSuccess:     true,
		// 	ImagePullSuccess:         true,
		// 	ContainerStartSuccess:    true,
		// 	ContainerCreateSuccess:   true,
		// 	ContainerInspectSuccess:  true,
		// 	CopyFromContainerSuccess: true,
		// 	ContainerStatPathSuccess: true,
		// }), errors.New("CopyFromContainerFailure")},
	}

	fmt.Println("RUN WITH", dir.Path())
	// defer dir.Remove()
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			runner := Runner{}
			runner.docker = tc.Client

			err := runner.setup(config.Suite{}, config.Run{})
			fmt.Println(err)
		})
	}

}
