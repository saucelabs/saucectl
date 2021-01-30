package legacydocker

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/config"
	"testing"

	"github.com/saucelabs/saucectl/cli/mocks"
	"gotest.tools/v3/fs"
)

func TestDockerRunnerSetup(t *testing.T) {
	type PassFailCase struct {
		Name          string
		Client        *LegacyHandler
		ExpectedError error
	}

	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yaml", "foo: bar", fs.WithMode(0755)))

	cases := []PassFailCase{
		{"docker is not installed", CreateLegacyMock(&mocks.FakeClient{}), errors.New("docker is not installed")},
		{"Pulling fails", CreateLegacyMock(&mocks.FakeClient{
			ContainerListSuccess: true,
		}), errors.New("ImagePullFailure")},
		{"Creating container fails", CreateLegacyMock(&mocks.FakeClient{
			ContainerListSuccess: true,
			ImagePullSuccess:     true,
		}), errors.New("ContainerCreateFailure")},
		{"Copy from container fails", CreateLegacyMock(&mocks.FakeClient{
			ContainerListSuccess:    true,
			ImagePullSuccess:        true,
			ContainerStartSuccess:   true,
			ContainerCreateSuccess:  true,
			ContainerInspectSuccess: true,
		}), errors.New("CopyFromContainerFailure")},
		// {"Can not find container config", docker.CreateLegacyMock(&mocks.FakeClient{
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
			runner := LegacyRunner{}
			runner.docker = tc.Client

			err := runner.setup(config.Suite{}, config.Run{})
			fmt.Println(err)
		})
	}

}
