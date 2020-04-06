package run

import (
	"context"
	"fmt"
	"os"

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
			checkErr(Run(cmd, cli, args))
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

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	var configFile Configuration
	config, err := configFile.readFromFilePath(cfgFilePath)
	if err != nil {
		return err
	}

	ctx := context.Background()
	hasBaseImage, err := cli.Docker.HasBaseImage(ctx, config.Image.Base)
	if err != nil {
		return err
	}

	if hasBaseImage {
		if err := cli.Docker.PullBaseImage(ctx, config.Image.Base); err != nil {
			return err
		}
	}

	container, err := cli.Docker.StartContainer(ctx, config.Image.Base)
	if err != nil {
		return err
	}

	if err := cli.Docker.CopyTestFilesToContainer(ctx, container.ID, config.Files); err != nil {
		return err
	}

	exitCode, err := cli.Docker.ExecuteTest(ctx, container.ID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	os.Exit(exitCode)
	return nil
}
