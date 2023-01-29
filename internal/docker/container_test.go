package docker

import (
	"context"
	"fmt"
	"testing"

	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestPullImage(t *testing.T) {
	ctx := context.Background()
	fc := mocks.FakeClient{}
	handler := &Handler{client: &fc}
	c := &ContainerRunner{
		Ctx:    ctx,
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

func Example_getJobID() {
	fmt.Println(getJobID("https://app.saucelabs.com/tests/cb6741a1a119448a9760531024657967"))
	// Output: cb6741a1a119448a9760531024657967
}
