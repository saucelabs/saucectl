package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/saucelabs/saucectl/cmd/logs"
	"github.com/saucelabs/saucectl/cmd/run"
	"github.com/spf13/cobra"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	cmdUse  = `saucectl`
	cmdLong = `Some main description`

	rootCmd = &cobra.Command{
		Use:  cmdUse,
		Long: cmdLong,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(validateDockerDependency)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.saucectl.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	// rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file (e.g. ./.sauce/config.yaml")

	// add child commands
	rootCmd.AddCommand(
		run.NewCmdRun(),
		logs.NewCmdLogs(),
	)
}

func validateDockerDependency() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	_, err = cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(fmt.Errorf("docker is not installed or running on your system"))
	}
}
