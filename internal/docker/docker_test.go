package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestNewImagePullOptions(t *testing.T) {
	os.Setenv(RegistryUsernameEnvKey, "fake-user")
	os.Setenv(RegistryPasswordEnvKey, "fake-password")

	opts, err := NewImagePullOptions()
	if err != nil {
		t.Fail()
	}
	want := map[string]string{"username": "fake-user", "password": "fake-password"}
	value := map[string]string{}

	decoded, err := base64.URLEncoding.DecodeString(opts.RegistryAuth)
	assert.Nil(t, err)
	err = json.Unmarshal(decoded, &value)
	assert.Nil(t, err)
	assert.Equal(t, value, want)
}

func TestHasBaseImage(t *testing.T) {
	ctx := context.Background()
	fc := mocks.FakeClient{}
	handler := &Handler{client: &fc}

	fc.ImageListSuccess = true
	val, err := handler.HasBaseImage(ctx, "dummy-image")
	assert.Nil(t, err)
	assert.True(t, val)

	fc.ImageListSuccess = false
	val, err = handler.HasBaseImage(ctx, "dummy-image")
	assert.NotNil(t, err)
	assert.False(t, val)
}

func TestClient(t *testing.T) {
	handler, err := Create()
	assert.Nil(t, err)
	assert.NotNil(t, handler)
}

func TestPullImageBase(t *testing.T) {
	fc := mocks.FakeClient{}
	handler := &Handler{client: &fc}

	fc.ImagePullSuccess = false
	err := handler.PullImage(context.Background(), "dummy-name:dumm-tag")
	assert.NotNil(t, err)
}

func TestCreateMounts(t *testing.T) {
	cwd, _ := os.Getwd()
	want := []struct {
		Idx    int
		Source string
		Target string
	}{
		{Idx: 0, Source: "file1", Target: "dest/file1"},
		{Idx: 1, Source: "dir1/file2", Target: "dest/file2"},
		{Idx: 2, Source: "dir1/dir2/file3", Target: "dest/file3"},
		{Idx: 3, Source: "dir1/dir2/file3", Target: "dest/file3"},
	}

	var files []string
	for _, f := range want {
		files = append(files, f.Source)
	}
	dest := "dest/"
	mounts, _ := createMounts("fakeSuite", files, dest)
	assert.Len(t, mounts, len(want))
	for _, w := range want {
		m := mounts[w.Idx]
		assert.Equal(t, path.Join(cwd, w.Source), m.Source)
		assert.Equal(t, w.Target, m.Target)
	}
}

func TestStartContainer(t *testing.T) {
	project := cypress.Project{
		Cypress: cypress.Cypress{
			ConfigFile:  "../../../tests/e2e/cypress.json",
		},
	}
	mockDocker := mocks.FakeClient{
		ContainerCreateSuccess:     false,
		ContainerStartSuccess:      true,
		ContainerInspectSuccess:    true,
		ImageInspectWithRawSuccess: true,
		CopyToContainerFn: func(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error {
			return nil
		},
	}
	handler := Handler{
		client: &mockDocker,
	}

	var cont *container.ContainerCreateCreatedBody
	var err error

	// Buggy container start
	cont, err = handler.StartContainer(context.Background(), containerStartOptions{RootDir: project.RootDir})
	assert.NotNil(t, err)

	// Successfull container start
	mockDocker.ContainerCreateSuccess = true
	cont, err = handler.StartContainer(context.Background(), containerStartOptions{RootDir: project.RootDir})
	assert.Nil(t, err)
	assert.NotNil(t, cont)
}

func TestExecuteInContainer(t *testing.T) {
	mockDocker := mocks.FakeClient{
		ContainerExecCreateSuccess: true,
		ContainerExecAttachSuccess: true,
	}
	handler := Handler{
		client: &mockDocker,
	}

	IDResponse, hijackedResponse, err := handler.Execute(context.Background(), "dummy-container-id", []string{"npm", "dummy-command"}, map[string]string{})
	assert.Nil(t, err)
	assert.NotNil(t, hijackedResponse)
	assert.Equal(t, IDResponse.ID, "dummy-id")
}

func TestCopyFromContainer(t *testing.T) {
	defer func() {
		os.Remove("internal-file")
	}()
	client := &mocks.FakeClient{
		ContainerStatPathSuccess: true,
		CopyFromContainerSuccess: true,
	}
	handler := Handler{
		client: client,
	}

	// Working
	err := handler.CopyFromContainer(context.Background(), "dummy-container-id", "/dummy/source/internal-file", "./")
	assert.Nil(t, err)

	// Errored test
	client.CopyFromContainerSuccess = false
	err = handler.CopyFromContainer(context.Background(), "dummy-container-id", "/dummy/source/internal-file", "./")
	assert.NotNil(t, err)
}

func TestHandler_IsInstalled(t *testing.T) {
	fakeVersion := types.Version{
		Platform: struct {
			Name string
		}{},
		Components: nil,
		Version:    "1.2.3",
	}

	type fields struct {
		client CommonAPIClient
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "expect installed",
			fields: fields{client: &mocks.FakeClient{
				ServerVersionFn: func(ctx context.Context) (types.Version, error) {
					return fakeVersion, nil
				},
			}},
			want: true,
		},
		{
			name: "expect not-installed",
			fields: fields{client: &mocks.FakeClient{
				ServerVersionFn: func(ctx context.Context) (types.Version, error) {
					return fakeVersion, errors.New("better expect me")
				},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{
				client: tt.fields.client,
			}
			if got := handler.IsInstalled(); got != tt.want {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}
