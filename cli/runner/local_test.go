package runner

import (
	"errors"
	"fmt"
	"testing"

	"github.com/saucelabs/saucectl/cli/docker"
	"github.com/saucelabs/saucectl/cli/mocks"
	"gotest.tools/v3/fs"
)

type PassFailCase struct {
	Name          string
	Client        *docker.Handler
	ExpectedError error
}

func TestLocalRunnerSetup(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yaml", "foo: bar", fs.WithMode(0755)))

	cases := []PassFailCase{
		{"Docker is not installed", docker.CreateMock(&mocks.FakeClient{}), errors.New("Docker is not installed")},
		{"Pulling fails", docker.CreateMock(&mocks.FakeClient{
			ContainerListSuccess: true,
		}), errors.New("ImagePullFailure")},
		{"Creating container fails", docker.CreateMock(&mocks.FakeClient{
			ContainerListSuccess: true,
			ImagePullSuccess:     true,
		}), errors.New("ContainerCreateFailure")},
		{"Copy from container fails", docker.CreateMock(&mocks.FakeClient{
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
			runner := localRunner{}
			runner.docker = tc.Client
			runner.tmpDir = dir.Path()

			err := runner.Setup()
			fmt.Println(err)
		})
	}

}
