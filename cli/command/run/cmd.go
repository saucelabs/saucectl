package run

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/spf13/cobra"
)

var (
	runUse     = "run ./.sauce/config.yaml"
	runShort   = "Run a test on Sauce Labs"
	runLong    = `Some long description`
	runExample = "saucectl run ./.sauce/config.yaml"

	defaultLogFir = "<cwd>/logs"

	cfgFilePath string
	cfgLogDir   string
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Run Command")
			exitCode, err := Run(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	cmd.Flags().StringVarP(&cfgFilePath, "config", "c", "./.sauce/config.yml", "config file (e.g. ./.sauce/config.yaml")
	cmd.Flags().StringVarP(&cfgLogDir, "logDir", "l", defaultLogFir, "log path")

	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) (int, error) {
	// Todo(Christian) write argument parser/validator
	if cfgLogDir == defaultLogFir {
		pwd, _ := os.Getwd()
		cfgLogDir = filepath.Join(pwd, "logs")
	}

	log.Info().Msgf("Read config file: %s", cfgFilePath)
	configObject, err := config.NewJobConfiguration(cfgFilePath)
	if err != nil {
		return 1, err
	}

	tr, err := runner.New(configObject, cli)
	if err != nil {
		return 1, err
	}

	log.Info().Msg("Setup test environment")
	if err := tr.Setup(); err != nil {
		return 1, err
	}

	log.Info().Msg("Start tests")
	exitCode, err := tr.Run()
	if err != nil {
		return 1, err
	}

	log.Info().Msg("Teardown environment")
	if err != tr.Teardown(cfgLogDir) {
		return 1, err
	}

	log.Info().
		Int("ExitCode", exitCode).
		Msg("Command Finished")

	return exitCode, nil
}
