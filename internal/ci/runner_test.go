package ci

import (
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/runner"

	"github.com/saucelabs/saucectl/internal/fleet"

	"github.com/stretchr/testify/assert"
)

type FakeSequencer struct {
	fleet.Sequencer
}

func TestTeardownCopyFiles(t *testing.T) {
	oldMethod := copyFile
	var files []string
	copyFile = func(src string, dst string) error {
		files = append(files, src)
		return nil
	}
	jobConfig := config.Project{}
	cli := &command.SauceCtlCli{}
	seq := FakeSequencer{}
	r, err := NewRunner(jobConfig, cli, seq, config.RunnerConfiguration{}, nil)
	assert.Equal(t, err, nil)
	runner.LogFiles = []string{"/foo/bar/", "/bar/foo/"}
	r.teardown("")
	assert.Equal(t, files, []string{"/foo/bar/", "/bar/foo/"})
	copyFile = oldMethod
}

func TestTeardownSkipped(t *testing.T) {
	jobConfig := config.Project{}
	cli := &command.SauceCtlCli{}
	seq := FakeSequencer{}
	r, err := NewRunner(jobConfig, cli, seq, config.RunnerConfiguration{}, nil)
	assert.Equal(t, err, nil)
	runner.LogFiles = []string{"/foo/bar/", "/bar/foo/"}
	err = r.teardown("some/path/")
	assert.Equal(t, err, nil)
}

func TestRunBeforeExec(t *testing.T) {
	jobConfig := config.Project{}
	cli := &command.SauceCtlCli{}
	seq := FakeSequencer{}
	r, err := NewRunner(jobConfig, cli, seq, config.RunnerConfiguration{}, nil)
	assert.Equal(t, err, nil)
	err = r.beforeExec(jobConfig.BeforeExec)
	assert.Equal(t, err, nil)
}
