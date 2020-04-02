package logs

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	cmdUse   = "saucectl logs --job <jobId>"
	cmdShort = "A brief description of your application"
	cmdLong  = `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`
	cmdExample = "saucectl --job <jobId>"
)

// Execute runs the command
func Execute() {
	fmt.Println("saucectl logs: Nothing to run yet")
}

// NewCmdLogs creates the `logs` command
func NewCmdLogs() *cobra.Command {
	cmd := &cobra.Command{
		Use:     cmdUse,
		Short:   cmdShort,
		Long:    cmdLong,
		Example: cmdExample,
	}

	// cobra.OnInitialize(initConfig)
	// cmd.Flags().StringVarP("jobId", "job id", "j", "", "")
	return cmd
}
