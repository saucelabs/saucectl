package cmd

import (
	"fmt"
	"os"

	"github.com/saucelabs/saucectl/cmd/logs"
	"github.com/saucelabs/saucectl/cmd/run"
	"github.com/spf13/cobra"
)

var (
	cmdUse  = `saucectl`
	cmdLong = `Some main description`

	rootCmd = &cobra.Command{
		Use: cmdUse,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("saucectl: Nothing to run yet")
}

func init() {
	// cobra.OnInitialize(initConfig)

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
