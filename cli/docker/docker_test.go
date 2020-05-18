package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

var ctx = context.Background()

type PassFailCase struct {
	Name           string
	Client         ClientInterface
	JobConfig      *config.JobConfiguration
	ExpectedError  error
	ExpectedResult interface{}
}

func TestValidateDependency(t *testing.T) {
	cases := []PassFailCase{
		{"Docker is not installed", &mocks.FakeClient{}, nil, errors.New("ContainerListFailure"), nil},
		{"Docker is intalled", &mocks.FakeClient{ContainerListSuccess: true}, nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{client: tc.Client}
			err := handler.ValidateDependency()
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestHasBaseImage(t *testing.T) {
	cases := []PassFailCase{
		{"failing command", &mocks.FakeClient{}, nil, errors.New("ImageListFailure"), false},
		{"passing command", &mocks.FakeClient{ImageListSuccess: true}, nil, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			hasBaseImage, err := handler.HasBaseImage(ctx, "foobar")
			assert.Equal(t, err, tc.ExpectedError)
			assert.Equal(t, hasBaseImage, tc.ExpectedResult)
		})
	}
}

func TestPullBaseImage(t *testing.T) {
	jobConfig := config.JobConfiguration{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"failing command", &mocks.FakeClient{}, &jobConfig, errors.New("ImagePullFailure"), nil},
		// {"passing command", &mocks.FakeClient{ImagePullSuccess: true}, nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			err := handler.PullBaseImage(ctx, *tc.JobConfig)
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestGetImageFlavorDefault(t *testing.T) {
	jobConfig := config.JobConfiguration{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"get image flavor", &mocks.FakeClient{}, &jobConfig, errors.New("Wrong flavor name"), false},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			baseImage := handler.GetImageFlavor(*tc.JobConfig)
			assert.Equal(t, baseImage, "foobar:latest")
		})
	}
}

func TestGetImageFlavorVersioned(t *testing.T) {
	jobConfig := config.JobConfiguration{
		Image: config.ImageDefinition{Base: "foobar", Version: "barfoo"},
	}
	cases := []PassFailCase{
		{"get image flavor", &mocks.FakeClient{}, &jobConfig, errors.New("Wrong flavor name"), false},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			baseImage := handler.GetImageFlavor(*tc.JobConfig)
			assert.Equal(t, baseImage, "foobar:barfoo")
		})
	}
}

func TestStartContainer(t *testing.T) {
	failureResult := container.ContainerCreateCreatedBody{}
	jobConfig := config.JobConfiguration{
		Image: config.ImageDefinition{Base: "foobar"},
		Capabilities: []config.Capabilities{
			{BrowserName: "chrome"},
		},
	}
	jobConfigWithoutCaps := config.JobConfiguration{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"failing to create container", &mocks.FakeClient{}, &jobConfig, errors.New("ContainerCreateFailure"), failureResult},
		{"failing to start container", &mocks.FakeClient{
			ContainerCreateSuccess: true,
		}, &jobConfig, errors.New("ContainerStartFailure"), failureResult},
		{"failing to inspect container", &mocks.FakeClient{
			ContainerCreateSuccess: true,
			ContainerStartSuccess:  true,
		}, &jobConfig, errors.New("ContainerInspectFailure"), failureResult},
		{"successful execution", &mocks.FakeClient{
			ContainerCreateSuccess:  true,
			ContainerStartSuccess:   true,
			ContainerInspectSuccess: true,
		}, &jobConfig, nil, failureResult},
		{"successful execution without caps", &mocks.FakeClient{
			ContainerCreateSuccess:  true,
			ContainerStartSuccess:   true,
			ContainerInspectSuccess: true,
		}, &jobConfigWithoutCaps, nil, failureResult},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			_, err := handler.StartContainer(ctx, *tc.JobConfig)
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestCopyTestFilesToContainer(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)))
	defer dir.Remove()

	cases := []PassFailCase{
		{"failing attempt to copy", &mocks.FakeClient{}, nil, errors.New("CopyToContainerFailure"), nil},
		{"passing command", &mocks.FakeClient{CopyToContainerSuccess: true}, nil, nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			testfiles := []string{
				dir.Path() + "/*.foo.js",
				dir.Path() + "/*.bar.js",
			}
			err := handler.CopyTestFilesToContainer(ctx, "containerId", testfiles, "/foo/bar")
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestCopyFromContainer(t *testing.T) {
	type PassFailCaseWithArgument struct {
		PassFailCase
		DstPath string
	}
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)))
	defer dir.Remove()
	srcFile := dir.Path() + "/some.foo.js"
	targetFile := dir.Path() + "/some.other.foo.js"

	cases := []PassFailCaseWithArgument{
		{PassFailCase{"not existing target dir", &mocks.FakeClient{}, nil, errors.New("invalid output path: directory /foo does not exist"), nil}, "/foo/bar"},
		{PassFailCase{"failure when getting stat info", &mocks.FakeClient{}, nil, errors.New("ContainerStatPathFailure"), nil}, targetFile},
		{PassFailCase{"failure when copying from container", &mocks.FakeClient{
			ContainerStatPathSuccess: true,
		}, nil, errors.New("CopyFromContainerFailure"), nil}, targetFile},
		{PassFailCase{"successful attempt", &mocks.FakeClient{
			ContainerStatPathSuccess: true,
			CopyFromContainerSuccess: true,
		}, nil, nil, nil}, targetFile},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			err := handler.CopyFromContainer(ctx, "containerId", srcFile, tc.DstPath)
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestExecute(t *testing.T) {
	cases := []PassFailCase{
		{"failing to create exec", &mocks.FakeClient{}, nil, errors.New("ContainerExecCreateFailure"), nil},
		{"failing to create attach", &mocks.FakeClient{
			ContainerExecCreateSuccess: true,
		}, nil, errors.New("ContainerExecAttachFailure"), nil},
		{"successful call", &mocks.FakeClient{
			ContainerExecCreateSuccess: true,
			ContainerExecAttachSuccess: true,
		}, nil, nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			_, _, err := handler.Execute(ctx, "containerId", []string{"npm", "test"})
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestExecuteExecuteInspect(t *testing.T) {
	cases := []PassFailCase{
		{"failing to inspect", &mocks.FakeClient{}, nil, errors.New("ContainerExecInspectFailure"), 1},
		{"successful call", &mocks.FakeClient{
			ContainerExecInspectSuccess: true,
		}, nil, nil, 0},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			exitCode, err := handler.ExecuteInspect(ctx, "containerId")
			assert.Equal(t, err, tc.ExpectedError)
			assert.Equal(t, exitCode, tc.ExpectedResult)
		})
	}
}

func TestContainerStop(t *testing.T) {
	cases := []PassFailCase{
		{"failing to inspect", &mocks.FakeClient{}, nil, errors.New("ContainerStopFailure"), nil},
		{"successful call", &mocks.FakeClient{
			ContainerStopSuccess: true,
		}, nil, nil, 0},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			err := handler.ContainerStop(ctx, "containerId")
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestContainerRemove(t *testing.T) {
	cases := []PassFailCase{
		{"failing to inspect", &mocks.FakeClient{}, nil, errors.New("ContainerRemoveFailure"), nil},
		{"successful call", &mocks.FakeClient{
			ContainerRemoveSuccess: true,
		}, nil, nil, 0},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			err := handler.ContainerRemove(ctx, "containerId")
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}
