package authoring

import "github.com/spf13/cobra"

// TestCasesCommand is the `authoring testcases` subgroup. It defines no
// pre-run of its own: cobra runs only the closest PersistentPreRun in the
// chain, so omitting it here lets the root's setup and entitlement gate run
// for every descendant.
func TestCasesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "testcases",
		Aliases:      []string{"testcase", "tc"},
		Short:        "Inspect, run, author and export AI-authored test cases",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		TestCasesListCommand(),
		TestCasesGetCommand(),
		TestCasesDeleteCommand(),
		TestCasesRenameCommand(),
		TestCasesRunCommand(),
		TestCasesListRunsCommand(),
		TestCasesGetRunCommand(),
		TestCasesListTagsCommand(),
		TestCasesGenerateCommand(),
		TestCasesGenerateStatusCommand(),
		TestCasesCodeCommand(),
		TestCasesListCodeTargetsCommand(),
	)

	return cmd
}
