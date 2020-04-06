package run

import (
	"context"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
			checkErr(Run(cmd, args))
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
func Run(cmd *cobra.Command, args []string) error {
	var configFile Configuration
	config, err := configFile.readFromFilePath(cfgFilePath)

	if err != nil {
		return err
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	listFilters := filters.NewArgs(
		filters.Arg("reference", config.Image.Base))
	options := types.ImageListOptions{
		All:     true,
		Filters: listFilters,
	}
	ctx := context.Background()
	images, err := cli.ImageList(ctx, options)
	if err != nil {
		return err
	}

	// pull image if not on machine
	if len(images) == 0 {
		options := types.ImagePullOptions{}
		responseBody, err := cli.ImagePull(ctx, config.Image.Base, options)

		if err != nil {
			return err
		}

		defer responseBody.Close()
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: config.Image.Base,
		Env:   []string{"SAUCE_USERNAME", "SAUCE_ACCESS_KEY"},
		Tty:   true,
	}, nil, nil, "")
	if err != nil {
		return err
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	// We need to check the tty _before_ we do the ContainerExecCreate, because
	// otherwise if we error out we will leak execIDs on the server (and
	// there's no easy way to clean those up). But also in order to make "not
	// exist" errors take precedence we do a dummy inspect first.
	if _, err := cli.ContainerInspect(ctx, resp.ID); err != nil {
		return err
	}

	// execConfig := &types.ExecConfig{
	// 	Cmd: []string{"cd", "/home/runner", "&&", "npm", "test"},
	// }

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return err
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	return nil
}
