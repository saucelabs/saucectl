package commands

import (
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/command/new"
	"github.com/saucelabs/saucectl/cli/command/run"
	"github.com/saucelabs/saucectl/cli/command/signup"
	"github.com/spf13/cobra"
)

// AddCommands attaches commands to cli
func AddCommands(cmd *cobra.Command, cli *command.SauceCtlCli) {
	cmd.AddCommand(
		new.Command(cli),
		run.Command(cli),
		signup.Command(cli),
	)
}
