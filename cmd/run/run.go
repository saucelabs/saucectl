package run

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cfgFile string

var (
	runUse     = "run ./.sauce/config.yaml"
	runShort   = "Run a test on Sauce Labs"
	runLong    = `Some long description`
	runExample = "run ./.sauce/config.yaml"
)

// NewCmdRun creates the `run` command
func NewCmdRun() *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			Run()
		},
	}

	// cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	// cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.saucectl.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file (e.g. ./.sauce/config.yaml")

	// cmd.AddCommand()

	return cmd
}

// Run runs the command
func Run() {
	fmt.Println("Run Sauce test")
}
