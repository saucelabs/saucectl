package main

import (
	"fmt"
	"os"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/command/commands"
	"github.com/spf13/cobra"
)

var (
	cmdUse   = "saucectl [OPTIONS] COMMAND [ARG...]"
	cmdShort = "saucectl"
	cmdLong  = "Some main description"
)

func main() {
	saucectlCli, err := command.NewSauceCtlCli()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmd := &cobra.Command{
		Use:              cmdUse,
		Short:            cmdShort,
		Long:             cmdLong,
		TraverseChildren: true,
	}

	commands.AddCommands(cmd, saucectlCli)
	cmd.Execute()
}
