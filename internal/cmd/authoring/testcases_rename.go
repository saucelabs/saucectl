package authoring

import (
	"fmt"

	"github.com/spf13/cobra"
)

// TestCasesRenameCommand is `authoring testcases rename`.
func TestCasesRenameCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "rename <id> <name>",
		Short:        "Rename a test case",
		Example:      `  saucectl authoring testcases rename 6a882c1dc8b4482c166e96c9 "Checkout - add two items"`,
		SilenceUsage: true,
		Args:         requireArgs("id", "name"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			tc, err := testCaseService.RenameTestCase(cmd.Context(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("failed to rename test case: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(tc)
			}
			fmt.Printf("Renamed test case %s to %q.\n", tc.ID, tc.Name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}
