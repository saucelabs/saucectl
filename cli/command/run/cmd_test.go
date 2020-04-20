package run

import (
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/stretchr/testify/assert"
)

func TestNewRunCommand(t *testing.T) {
	cli := command.SauceCtlCli{}
	cmd := NewRunCommand(&cli)
	assert.Equal(t, cmd.Use, runUse)
}
