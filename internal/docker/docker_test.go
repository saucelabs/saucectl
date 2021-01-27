package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/docker/docker/api/types"
	"io"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/api/types/container"

	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/saucelabs/saucectl/internal/cypress"
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

func TestImageFlavor(t *testing.T) {
	tests := []struct {
		Image string
		Tag   string
		Want  string
	}{
		{"dummy-image", "latest", "dummy-image:latest"},
		{"dummy-image", "", "dummy-image:latest"},
		{"dummy-image", "custom-tag", "dummy-image:custom-tag"},
	}

	handler := Handler{}
	for _, tt := range tests {
		img := config.Image{Name: tt.Image, Tag: tt.Tag}
		have := handler.GetImageFlavor(img)
		assert.Equal(t, have, tt.Want)
	}
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

func TestValidateDependency(t *testing.T) {
	fc := mocks.FakeClient{}
	handler := &Handler{client: &fc}

	fc.ContainerListSuccess = true
	err := handler.ValidateDependency()
	assert.Nil(t, err)

	fc.ContainerListSuccess = false
	err = handler.ValidateDependency()
	assert.NotNil(t, err)
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
	err := handler.PullBaseImage(context.Background(), config.Image{Name: "dummy-name", Tag: "dummy-tag"})
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
	mounts, _ := createMounts(files, dest)
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
			ProjectPath: "../../../tests/e2e/",
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
	cont, err = handler.StartContainer(context.Background(), []string{project.Cypress.ConfigFile, project.Cypress.ProjectPath}, config.Docker{})
	assert.NotNil(t, err)

	// Successfull container start
	mockDocker.ContainerCreateSuccess = true
	cont, err = handler.StartContainer(context.Background(), []string{project.Cypress.ConfigFile, project.Cypress.ProjectPath}, config.Docker{})
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