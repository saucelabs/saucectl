package logs

import (
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/stretchr/testify/assert"
)

func TestRunCmd(t *testing.T) {
	assert.Equal(t, Run(), 0)
}

func TestNewLogsCommand(t *testing.T) {
	cli := command.SauceCtlCli{}
	cmd := NewLogsCommand(&cli)
	assert.Equal(t, cmd.Use, logsUse)
}
