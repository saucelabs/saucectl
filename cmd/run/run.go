package run

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

var (
	runUse     = "run ./.sauce/config.yaml"
	runShort   = "Run a test on Sauce Labs"
	runLong    = `Some long description`
	runExample = "run ./.sauce/config.yaml"

	cfgFilePath string
)

// NewCmdRun creates the `run` command
func NewCmdRun() *cobra.Command {
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
		panic(err)
	}

	listFilters := filters.NewArgs(
		filters.Arg("reference", config.Image.Base))
	options := types.ImageListOptions{
		All:     true,
		Filters: listFilters,
	}
	images, err := cli.ImageList(context.Background(), options)
	if err != nil {
		panic(err)
	}

	if len(images) > 0 {
		fmt.Println("I have the image")
		return nil
	}

	fmt.Println("PULL IT")
	return nil
}
