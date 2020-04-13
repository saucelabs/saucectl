package run

import (
	"context"
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/docker"
	"github.com/stretchr/testify/assert"
)

func TestExportArtifacts(t *testing.T) {
	fakeClient := docker.CreateMock()
	cli := command.SauceCtlCli{
		Docker: fakeClient,
	}

	err := ExportArtifacts(context.Background(), &cli, "containerId", "./")
	assert.Equal(t, err, nil)
}
