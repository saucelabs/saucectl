package runner

import (
	"fmt"
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestNewLocalRunner(t *testing.T) {
	config := config.Project{}
	cli := command.SauceCtlCli{}

	runner, err := NewDockerRunner(config, &cli)
	if err != nil {
		t.Fatal(err)
	}

	runnerType := fmt.Sprintf("%T", runner)
	assert.Equal(t, "*runner.DockerRunner", runnerType)
}

func TestNewCIRunner(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yaml", "targetDir: /foo/bar", fs.WithMode(0755)))
	defer dir.Remove()
	RunnerConfigPath = dir.Path() + "/config.yaml"

	config := config.Project{}
	cli := command.SauceCtlCli{}

	runner, err := NewCIRunner(config, &cli)
	if err != nil {
		t.Fatal(err)
	}

	runnerType := fmt.Sprintf("%T", runner)
	assert.Equal(t, "*runner.ciRunner", runnerType)
}
