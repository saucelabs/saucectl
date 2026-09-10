package authoring

import (
	"fmt"

	"github.com/spf13/cobra"
)

// TestCasesListCodeTargetsCommand is `authoring testcases list-code-targets`.
func TestCasesListCodeTargetsCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "list-code-targets <id>",
		Aliases:      []string{"code-targets"},
		Short:        "List the languages and frameworks a test case can be exported to",
		Example:      `  saucectl authoring testcases list-code-targets 6a882c1dc8b4482c166e96c9`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			targets, err := testCaseService.CodeTargets(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to list code targets: %w", err)
			}
			if out == JSONOutput {
				if targets == nil {
					targets = []string{}
				}
				return renderJSON(targets)
			}
			if len(targets) == 0 {
				fmt.Println("No code targets available for this test case.")
				return nil
			}
			for _, t := range targets {
				fmt.Println(t)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}
