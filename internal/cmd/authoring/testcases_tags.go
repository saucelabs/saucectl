package authoring

import (
	"fmt"

	"github.com/spf13/cobra"
)

// TestCasesListTagsCommand is `authoring testcases list-tags`. Tags are
// returned exactly as the service holds them: case-sensitive, not folded, not
// deduplicated — "Login" and "login" are two tags.
func TestCasesListTagsCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "list-tags",
		Aliases:      []string{"tags"},
		Short:        "List every tag in use across the organisation's test cases",
		Example:      `  saucectl authoring testcases list-tags`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			tags, err := testCaseService.ListTags(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list tags: %w", err)
			}
			if out == JSONOutput {
				if tags == nil {
					tags = []string{}
				}
				return renderJSON(tags)
			}
			if len(tags) == 0 {
				fmt.Println("No tags found.")
				return nil
			}
			for _, tag := range tags {
				fmt.Println(tag)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}
