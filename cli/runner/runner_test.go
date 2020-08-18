package runner

import (
	"fmt"
	"os"
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestNewLocalRunner(t *testing.T) {
	// Simulate local runner, even if this test is run in CI.
	os.Unsetenv("CI")
	os.Unsetenv("BUILD_NUMBER")

	config := config.Project{}
	cli := command.SauceCtlCli{}

	runner, err := New(config, &cli)
	if err != nil {
		t.Fatal(err)
	}

	runnerType := fmt.Sprintf("%T", runner)
	assert.Equal(t, "*runner.localRunner", runnerType)
}

func TestNewCIRunner(t *testing.T) {
	// Pretend we are in a CI environment.
	os.Setenv("CI", "1")

	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yaml", "targetDir: /foo/bar", fs.WithMode(0755)))
	defer dir.Remove()
	RunnerConfigPath = dir.Path() + "/config.yaml"

	config := config.Project{}
	cli := command.SauceCtlCli{}

	runner, err := New(config, &cli)
	if err != nil {
		t.Fatal(err)
	}

	runnerType := fmt.Sprintf("%T", runner)
	assert.Equal(t, "*runner.ciRunner", runnerType)
}
