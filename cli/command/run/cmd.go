package run

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/saucelabs/saucectl/cli/command"
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

	cmd.Flags().StringVarP(&cfgFilePath, "config", "c", "", "config file (e.g. ./.sauce/config.yaml")
	cmd.Flags().StringVarP(&cfgLogDir, "logDir", "l", defaultLogFir, "log path")

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
	// Todo(Christian) write argument parser/validator
	if cfgLogDir == defaultLogFir {
		pwd, _ := os.Getwd()
		cfgLogDir = filepath.Join(pwd, "logs")
	}

	cli.Logger.Info().Msg("Read config file")
	var configFile Configuration
	config, err := configFile.readFromFilePath(cfgFilePath)
	if err != nil {
		return err
	}

	if true {
		return runFromHostMachine(cli, config)
	}

	return runFromContainer(cli, config)
}

func runFromHostMachine(cli *command.SauceCtlCli, config Configuration) error {
	ctx := context.Background()
	startTime := makeTimestamp()
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

	// wait until Xvfb started
	// ToDo(Christian): make this dynamic
	time.Sleep(1 * time.Second)

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Copy files to container")
	if err := cli.Docker.CopyTestFilesToContainer(ctx, container.ID, config.Files); err != nil {
		return err
	}

	var (
		out, stderr io.Writer
		in          io.ReadCloser
	)
	out = cli.Out()
	stderr = cli.Out()

	if err := cli.In().CheckTty(false, true); err != nil {
		return err
	}

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Run tests")
	createResp, attachResp, err := cli.Docker.ExecuteTest(ctx, container.ID)
	if err != nil {
		return err
	}

	defer attachResp.Close()

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		errCh <- func() error {
			streamer := ioStreamer{
				streams:      cli,
				inputStream:  in,
				outputStream: out,
				errorStream:  stderr,
				resp:         *attachResp,
				detachKeys:   "",
			}

			return streamer.stream(ctx)
		}()
	}()

	if err := <-errCh; err != nil {
		return err
	}

	exitCode, err := cli.Docker.ExecuteInspect(ctx, createResp.ID)
	if err != nil {
		return err
	}

	cli.Logger.Info().Int64("Duration", makeTimestamp()-startTime).Msg("Download artifatcs")
	if err := ExportArtifacts(ctx, cli, container.ID, cfgLogDir); err != nil {
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

func runFromContainer(cli *command.SauceCtlCli, config Configuration) error {
	return nil
}
