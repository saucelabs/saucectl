package authoring

import "github.com/spf13/cobra"

// VariablesCommand is the `authoring variables` subgroup. No pre-run of its
// own, so the root's runs (see TestCasesCommand).
func VariablesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "variables",
		Aliases:      []string{"variable", "var"},
		Short:        "Manage shared values available to authored tests",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		VariablesListCommand(),
		VariablesGetCommand(),
		VariablesCreateCommand(),
		VariablesUpdateCommand(),
		VariablesDeleteCommand(),
	)

	return cmd
}
