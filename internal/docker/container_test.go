package docker

import (
	"context"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)



func TestPullImage(t *testing.T) {
	ctx := context.Background()
	fc := mocks.FakeClient{}
	handler := &Handler{client: &fc}
	c := &ContainerRunner{
		Ctx: ctx,
		docker: handler,
	}
	err := c.pullImage("")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "no docker image specified")

	fc.HasBaseImageSuccess = true
	fc.ImagePullSuccess = true
	fc.ImageListSuccess = true
	err = c.pullImage("vrunoland:latest")
	assert.Nil(t, err)

	fc.HasBaseImageSuccess = false
	fc.ImageListSuccess = true
	fc.ImagePullSuccess = false
	fc.ImagesListEmpty = true
	err = c.pullImage("vrunoland:latest")
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "ImagePullFailure")
}


