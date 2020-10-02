package docker

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

var ctx = context.Background()

type PassFailCase struct {
	Name           string
	Client         CommonAPIClient
	JobConfig      *config.Project
	Suite          *config.Suite
	ExpectedError  error
	ExpectedResult interface{}
}

func TestValidateDependency(t *testing.T) {
	cases := []PassFailCase{
		{"Docker is not installed", &mocks.FakeClient{}, nil, nil, errors.New("ContainerListFailure"), nil},
		{"Docker is intalled", &mocks.FakeClient{ContainerListSuccess: true}, nil, nil, nil, nil},
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
		{"failing command", &mocks.FakeClient{}, nil, nil, errors.New("ImageListFailure"), false},
		{"passing command", &mocks.FakeClient{ImageListSuccess: true}, nil, nil, nil, true},
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

func TestGetImagePullOptionsUsesRegistryAuth(t *testing.T) {
	os.Setenv("REGISTRY_USERNAME", "registry-user")
	os.Setenv("REGISTRY_PASSWORD", "registry-pwd")
	jobConfig := config.Project{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"correct options", &mocks.FakeClient{}, &jobConfig, nil, errors.New("GetImagePullOptionsFailure"), nil},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			options, err := NewImagePullOptions()
			assert.Equal(t, err, nil)
			assert.NotEmpty(t, options.RegistryAuth)
		})
	}
	os.Unsetenv("REGISTRY_USERNAME")
	os.Unsetenv("REGISTRY_PASSWORD")
}

func TestGetImagePullOptionsDefault(t *testing.T) {
	jobConfig := config.Project{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"default options", &mocks.FakeClient{}, &jobConfig, nil, errors.New("GetImagePullOptionsFailure"), nil},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			options, err := NewImagePullOptions()
			assert.Equal(t, err, nil)
			assert.Equal(t, options.RegistryAuth, "")
		})
	}
}

func TestPullBaseImage(t *testing.T) {
	jobConfig := config.Project{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"failing command", &mocks.FakeClient{}, &jobConfig, nil, errors.New("ImagePullFailure"), nil},
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
	jobConfig := config.Project{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"get image flavor", &mocks.FakeClient{}, &jobConfig, nil, errors.New("Wrong flavor name"), false},
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
	jobConfig := config.Project{
		Image: config.ImageDefinition{Base: "foobar", Version: "barfoo"},
	}
	cases := []PassFailCase{
		{"get image flavor", &mocks.FakeClient{}, &jobConfig, nil, errors.New("Wrong flavor name"), false},
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
	jobConfig := config.Project{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	suite := config.Suite{Settings: config.Settings{BrowserName: "chrome"}}
	jobConfigWithoutCaps := config.Project{
		Image: config.ImageDefinition{Base: "foobar"},
	}
	cases := []PassFailCase{
		{"failing to create container", &mocks.FakeClient{}, &jobConfig, &suite, errors.New("ContainerCreateFailure"), failureResult},
		{"failing to start container", &mocks.FakeClient{
			ContainerCreateSuccess: true,
		}, &jobConfig, &suite, errors.New("ContainerStartFailure"), failureResult},
		{"failing to inspect container", &mocks.FakeClient{
			ContainerCreateSuccess: true,
			ContainerStartSuccess:  true,
		}, &jobConfig, &suite, errors.New("ContainerInspectFailure"), failureResult},
		{"successful execution", &mocks.FakeClient{
			ContainerCreateSuccess:  true,
			ContainerStartSuccess:   true,
			ContainerInspectSuccess: true,
		}, &jobConfig, &suite, nil, failureResult},
		{"successful execution without caps", &mocks.FakeClient{
			ContainerCreateSuccess:  true,
			ContainerStartSuccess:   true,
			ContainerInspectSuccess: true,
		}, &jobConfigWithoutCaps, &suite, nil, failureResult},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			handler := Handler{
				client: tc.Client,
			}
			_, err := handler.StartContainer(ctx, *tc.JobConfig, *tc.Suite)
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
		{PassFailCase{"not existing target dir", &mocks.FakeClient{}, nil, nil, errors.New("invalid output path: directory /foo does not exist"), nil}, "/foo/bar"},
		{PassFailCase{"failure when getting stat info", &mocks.FakeClient{}, nil, nil, errors.New("ContainerStatPathFailure"), nil}, targetFile},
		{PassFailCase{"failure when copying from container", &mocks.FakeClient{
			ContainerStatPathSuccess: true,
		}, nil, nil, errors.New("CopyFromContainerFailure"), nil}, targetFile},
		{PassFailCase{"successful attempt", &mocks.FakeClient{
			ContainerStatPathSuccess: true,
			CopyFromContainerSuccess: true,
		}, nil, nil, nil, nil}, targetFile},
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
		{"failing to create exec", &mocks.FakeClient{}, nil, nil, errors.New("ContainerExecCreateFailure"), nil},
		{"failing to create attach", &mocks.FakeClient{
			ContainerExecCreateSuccess: true,
		}, nil, nil, errors.New("ContainerExecAttachFailure"), nil},
		{"successful call", &mocks.FakeClient{
			ContainerExecCreateSuccess: true,
			ContainerExecAttachSuccess: true,
		}, nil, nil, nil, nil},
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
		{"failing to inspect", &mocks.FakeClient{}, nil, nil, errors.New("ContainerExecInspectFailure"), 1},
		{"successful call", &mocks.FakeClient{
			ContainerExecInspectSuccess: true,
		}, nil, nil, nil, 0},
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
		{"failing to inspect", &mocks.FakeClient{}, nil, nil, errors.New("ContainerStopFailure"), nil},
		{"successful call", &mocks.FakeClient{
			ContainerStopSuccess: true,
		}, nil, nil, nil, 0},
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
		{"failing to inspect", &mocks.FakeClient{}, nil, nil, errors.New("ContainerRemoveFailure"), nil},
		{"successful call", &mocks.FakeClient{
			ContainerRemoveSuccess: true,
		}, nil, nil, nil, 0},
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

func TestHandler_CopyToContainer(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)),
		fs.WithDir("subdir", fs.WithFile("some.subdir.js", "subdir")))
	defer dir.Remove()
	baseDir := filepath.Base(dir.Path())

	type fields struct {
		client CommonAPIClient
	}
	type args struct {
		ctx         context.Context
		containerID string
		srcFile     string
		targetDir   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "copy one file",
			fields: fields{&mocks.FakeClient{CopyToContainerFn: func(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error {
				expect := []string{
					"some.foo.js",
				}

				return expectTar(expect, content)
			}}},
			args:    args{ctx, "cid", dir.Join("some.foo.js"), "/foo/bar"},
			wantErr: false,
		},
		{
			name: "copy entire folder",
			fields: fields{&mocks.FakeClient{CopyToContainerFn: func(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error {
				expect := []string{
					baseDir + string(filepath.Separator),
					filepath.Join(baseDir, "some.foo.js"),
					filepath.Join(baseDir, "some.other.bar.js"),
					filepath.Join(baseDir, "subdir") + string(filepath.Separator),
					filepath.Join(baseDir, "subdir/some.subdir.js"),
				}

				return expectTar(expect, content)
			}}},
			args:    args{ctx, "cid", dir.Path(), "/foo/bar"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{
				client: tt.fields.client,
			}
			if err := handler.CopyToContainer(tt.args.ctx, tt.args.containerID, tt.args.srcFile, tt.args.targetDir); (err != nil) != tt.wantErr {
				t.Errorf("CopyToContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func expectTar(files []string, r io.Reader) error {
	ex := make(map[string]bool, len(files))
	for _, f := range files {
		ex[f] = true
	}

	var found []string
	// Open and iterate through the files in the archive.
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			log.Fatal(err)
		}
		found = append(found, hdr.Name)
	}

	if !reflect.DeepEqual(files, found) {
		return fmt.Errorf("expected %v but found %v", files, found)
	}

	return nil
}
