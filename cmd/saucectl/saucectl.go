package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/command/commands"
)

var (
	cmdUse   = "saucectl [OPTIONS] COMMAND [ARG...]"
	cmdShort = "saucectl"
	cmdLong  = "Some main description"
)

func main() {
	cli, err := command.NewSauceCtlCli()
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

	commands.AddCommands(cmd, cli)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
