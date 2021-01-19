package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
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

func TestStartContainer(t *testing.T) {
	//ctx := context.Background()
	//project := cypress.Project{}
	//handler := Handler{
	//
	//}
	//container, err := handler.StartContainer(ctx, project)
	//assert.Nil(t, err)
	//assert.NotNil(t, container)
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