package authoring

import "github.com/spf13/cobra"

// SchedulesCommand is the `authoring schedules` subgroup. No pre-run of its
// own, so the root's runs (see TestCasesCommand).
func SchedulesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "schedules",
		Aliases:      []string{"schedule", "test-schedules"},
		Short:        "Run test suites on a recurring schedule",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		SchedulesListCommand(),
		SchedulesGetCommand(),
		SchedulesCreateCommand(),
		SchedulesUpdateCommand(),
		SchedulesEnableCommand(),
		SchedulesDisableCommand(),
		SchedulesDeleteCommand(),
	)

	return cmd
}
