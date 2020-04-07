package run

import (
	"context"
	"os"
	"time"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/spf13/cobra"
)

var (
	runUse     = "run ./.sauce/config.yaml"
	runShort   = "Run a test on Sauce Labs"
	runLong    = `Some long description`
	runExample = "saucectl run ./.sauce/config.yaml"

	cfgFilePath string
)

// NewRunCommand creates the `run` command
func NewRunCommand(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			cli.Logger.Info().Msg("Start Run Command")
			checkErr(Run(cmd, cli, args))
			os.Exit(0)
		},
	}

	// cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	// cmd.PersistentFlags().StringVar(&cfgFilePath, "config", "", "config file (default is $HOME/.saucectl.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	cmd.Flags().StringVarP(&cfgFilePath, "config", "c", "", "config file (e.g. ./.sauce/config.yaml")

	return cmd
}

func checkErr(e error) {
	if e != nil {
		panic(e)
	}
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	startTime := makeTimestamp()
	ctx := context.Background()

	cli.Logger.Info().Msg("Read config file")
	var configFile Configuration
	config, err := configFile.readFromFilePath(cfgFilePath)
	if err != nil {
		return err
	}

	hasBaseImage, err := cli.Docker.HasBaseImage(ctx, config.Image.Base)
	if err != nil {
		return err
	}

	if !hasBaseImage {
		cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Pull base image")
		if err := cli.Docker.PullBaseImage(ctx, config.Image.Base); err != nil {
			return err
		}
	}

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Start container")
	container, err := cli.Docker.StartContainer(ctx, config.Image.Base)
	if err != nil {
		return err
	}

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Copy files to container")
	if err := cli.Docker.CopyTestFilesToContainer(ctx, container.ID, config.Files); err != nil {
		return err
	}

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Run tests")
	exitCode, err := cli.Docker.ExecuteTest(ctx, container.ID)
	if err != nil {
		return err
	}

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Stop container")
	if err := cli.Docker.ContainerStop(ctx, container.ID); err != nil {
		return err
	}

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Remove container")
	if err := cli.Docker.ContainerRemove(ctx, container.ID); err != nil {
		return err
	}

	cli.Logger.Info().
		Int64("Duration", makeTimestamp()-startTime).
		Int("ExitCode", exitCode).
		Msg("Command Finished")
	return nil
}
