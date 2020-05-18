package run

import (
	"testing"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestNewRunCommand(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yaml", "apiVersion: 1.2\nimage:\n  base: test", fs.WithMode(0755)))
	cli := command.SauceCtlCli{}
	cmd := Command(&cli)
	assert.Equal(t, cmd.Use, runUse)

	if err := cmd.Flags().Set("config", dir.Path()+"/config.yaml"); err != nil {
		t.Fatal(err)
	}

	var args []string
	exitCode, err := Run(cmd, &cli, args)

	assert.Equal(t, err, nil)
	assert.Equal(t, exitCode, 123)
}

func TestCheckErr(t *testing.T) {
	checkErr(nil)
}
