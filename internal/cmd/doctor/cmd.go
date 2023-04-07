package doctor

import (
	"os"

	"github.com/saucelabs/saucectl/internal/doctor"
	"github.com/spf13/cobra"
)

// Command creates the `doctor` command
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "Runs a series of checks to ensure that your environment is ready to run tests",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			if err := doctor.Verify(); err != nil {
				os.Exit(1)
			}
		},
	}

	return cmd
}
