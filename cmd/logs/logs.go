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

// Execute runs the command
func Execute() {
	fmt.Println("saucectl logs: stream logs from %s", jobID)
}

// NewCmdLogs creates the `logs` command
func NewCmdLogs() *cobra.Command {
	cmd := &cobra.Command{
		Use:     logsUse,
		Short:   logsShort,
		Long:    logsLong,
		Example: logsExample,
	}

	// cobra.OnInitialize(initConfig)
	cmd.Flags().StringVarP(&jobID, "job id", "j", "", "")
	return cmd
}
