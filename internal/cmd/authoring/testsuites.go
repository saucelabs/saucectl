package authoring

import "github.com/spf13/cobra"

// TestSuitesCommand is the `authoring testsuites` subgroup. No pre-run of its
// own, so the root's runs (see TestCasesCommand).
func TestSuitesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "testsuites",
		Aliases:      []string{"testsuite", "ts"},
		Short:        "Group test cases into suites",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		TestSuitesListCommand(),
		TestSuitesGetCommand(),
		TestSuitesCreateCommand(),
		TestSuitesUpdateCommand(),
		TestSuitesDeleteCommand(),
		TestSuitesRunCommand(),
	)

	return cmd
}
