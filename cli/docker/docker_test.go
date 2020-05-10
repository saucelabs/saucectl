package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

var ctx = context.Background()

type PassFailCase struct {
	Name           string
	Client         ClientInterface
	ExpectedError  error
	ExpectedResult interface{}
}

func TestValidateDependency(t *testing.T) {
	cases := []PassFailCase{
		{"Docker is not installed", &FakeClient{}, errors.New("ContainerListFailure"), nil},
		{"Docker is intalled", &FakeClient{ContainerListSuccess: true}, nil, nil},
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
		{"failing command", &FakeClient{}, errors.New("ImageListFailure"), false},
		{"passing command", &FakeClient{ImageListSuccess: true}, nil, true},
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
	cases := []PassFailCase{
		{"failing command", &FakeClient{}, errors.New("ImagePullFailure"), nil},
		// {"passing command", &FakeClient{ImagePullSuccess: true}, nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			c := config.JobConfiguration{
				Image: config.ImageDefinition{Base: "foobar"},
			}
			err := handler.PullBaseImage(ctx, c)
			assert.Equal(t, err, tc.ExpectedError)
		})
	}
}

func TestGetImageFlavorDefault(t *testing.T) {
	cases := []PassFailCase{
		{"get image flavor", &FakeClient{}, errors.New("Wrong flavor name"), false},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			c := config.JobConfiguration{
				Image: config.ImageDefinition{Base: "foobar"},
			}
			baseImage := handler.GetImageFlavor(c)
			assert.Equal(t, baseImage, "foobar:latest")
		})
	}
}

func TestGetImageFlavorVersioned(t *testing.T) {
	cases := []PassFailCase{
		{"get image flavor", &FakeClient{}, errors.New("Wrong flavor name"), false},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			c := config.JobConfiguration{
				Image: config.ImageDefinition{Base: "foobar", Version: "barfoo"},
			}
			baseImage := handler.GetImageFlavor(c)
			assert.Equal(t, baseImage, "foobar:barfoo")
		})
	}
}

func TestStartContainer(t *testing.T) {
	failureResult := container.ContainerCreateCreatedBody{}
	cases := []PassFailCase{
		{"failing to create container", &FakeClient{}, errors.New("ContainerCreateFailure"), failureResult},
		{"failing to start container", &FakeClient{
			ContainerCreateSuccess: true,
		}, errors.New("ContainerStartFailure"), failureResult},
		{"failing to inspect container", &FakeClient{
			ContainerCreateSuccess: true,
			ContainerStartSuccess:  true,
		}, errors.New("ContainerInspectFailure"), failureResult},
		{"successful execution", &FakeClient{
			ContainerCreateSuccess:  true,
			ContainerStartSuccess:   true,
			ContainerInspectSuccess: true,
		}, nil, failureResult},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			c := config.JobConfiguration{
				Image: config.ImageDefinition{Base: "foobar"},
			}
			_, err := handler.StartContainer(ctx, c)
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
		{"failing attempt to copy", &FakeClient{}, errors.New("CopyToContainerFailure"), nil},
		{"passing command", &FakeClient{CopyToContainerSuccess: true}, nil, nil},
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
		{PassFailCase{"not existing target dir", &FakeClient{}, errors.New("invalid output path: directory /foo does not exist"), nil}, "/foo/bar"},
		{PassFailCase{"failure when getting stat info", &FakeClient{}, errors.New("ContainerStatPathFailure"), nil}, targetFile},
		{PassFailCase{"failure when copying from container", &FakeClient{
			ContainerStatPathSuccess: true,
		}, errors.New("CopyFromContainerFailure"), nil}, targetFile},
		{PassFailCase{"successful attempt", &FakeClient{
			ContainerStatPathSuccess: true,
			CopyFromContainerSuccess: true,
		}, nil, nil}, targetFile},
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
		{"failing to create exec", &FakeClient{}, errors.New("ContainerExecCreateFailure"), nil},
		{"failing to create attach", &FakeClient{
			ContainerExecCreateSuccess: true,
		}, errors.New("ContainerExecAttachFailure"), nil},
		{"successful call", &FakeClient{
			ContainerExecCreateSuccess: true,
			ContainerExecAttachSuccess: true,
		}, nil, nil},
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
		{"failing to inspect", &FakeClient{}, errors.New("ContainerExecInspectFailure"), 1},
		{"successful call", &FakeClient{
			ContainerExecInspectSuccess: true,
		}, nil, 0},
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
		{"failing to inspect", &FakeClient{}, errors.New("ContainerStopFailure"), nil},
		{"successful call", &FakeClient{
			ContainerStopSuccess: true,
		}, nil, 0},
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
		{"failing to inspect", &FakeClient{}, errors.New("ContainerRemoveFailure"), nil},
		{"successful call", &FakeClient{
			ContainerRemoveSuccess: true,
		}, nil, 0},
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
