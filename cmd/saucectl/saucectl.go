package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/command/commands"
	"github.com/saucelabs/saucectl/cli/version"
)

var (
	cmdUse   = "saucectl [OPTIONS] COMMAND [ARG...]"
	cmdShort = "saucectl"
	cmdLong  = "Some main description"
)

func main() {
	cli := command.NewSauceCtlCli()
	cmd := &cobra.Command{
		Use:              cmdUse,
		Short:            cmdShort,
		Long:             cmdLong,
		TraverseChildren: true,
		Version:          fmt.Sprintf("%s\n(build %s)", version.Version, version.GitCommit),
	}

	cmd.SetVersionTemplate("saucectl version {{.Version}}\n")
	cmd.Flags().BoolP("version", "v", false, "print version")

	verbosity := cmd.PersistentFlags().Bool("verbose", false, "turn on verbose logging")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		setupLogging(*verbosity)
		return nil
	}

	commands.AddCommands(cmd, cli)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func setupLogging(verbose bool) {
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	if verbose {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
}
