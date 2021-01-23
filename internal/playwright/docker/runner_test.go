package docker

import (
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/mocks"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPreliminarySteps_Basic(t *testing.T) {
	runner := Runner{Project: playwright.Project{Playwright: playwright.Playwright{Version: "5.6.2"}}}
	assert.Nil(t, runner.defineDockerImage())
}

func TestPreliminarySteps_DefinedImage(t *testing.T) {
	runner := Runner{
		Project: playwright.Project{
			Docker: config.Docker{
				Image: config.Image{Name: "dummy-image", Tag: "dummy-tag"},
			},
		},
	}
	assert.Nil(t, runner.defineDockerImage())
}

func TestPreliminarySteps_NoDefinedImageNoCypressVersion(t *testing.T) {
	want := "no cypress version provided"
	runner := Runner{}
	err := runner.defineDockerImage()
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), want)
}

func TestNewRunner(t *testing.T) {
	p := playwright.Project{}
	cli := command.NewSauceCtlCli()
	r, err := New(p, cli)
	assert.NotNil(t, r)
	assert.Nil(t, err)
}

func TestTearDown(t *testing.T) {
	docker := &mocks.FakeClient{
		ContainerInspectSuccess: true,
		ContainerStopSuccess:    true,
		ContainerRemoveSuccess:  true,
	}
	runner := Runner{
		docker: &Handler{
			client: docker,
		},
	}
	assert.Nil(t, runner.teardown("logs"))
}