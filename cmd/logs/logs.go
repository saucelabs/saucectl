package logs

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	logsUse     = "logs --job <jobId>"
	logsShort   = "stream logs from jobs"
	logsLong    = `Some long description`
	logsExample = "saucectl --job <jobId>"

	jobID string
)

// NewCmdLogs creates the `logs` command
func NewCmdLogs() *cobra.Command {
	cmd := &cobra.Command{
		Use:     logsUse,
		Short:   logsShort,
		Long:    logsLong,
		Example: logsExample,
		Run: func(cmd *cobra.Command, args []string) {
			Run()
		},
	}

	// cobra.OnInitialize(initConfig)
	// cmd.Flags().StringVarP(&jobID, "job id", "j", "", "")
	return cmd
}

// Run should run command
func Run() {
	fmt.Println(("Run logs command"))
}
