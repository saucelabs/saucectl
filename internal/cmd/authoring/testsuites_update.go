package authoring

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestSuitesUpdateCommand is `authoring testsuites update`.
func TestSuitesUpdateCommand() *cobra.Command {
	var out string
	var opts authoring.UpdateTestSuiteOptions

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a test suite's name, tags or membership",
		Long: `Update a test suite. --test-case replaces the membership wholesale; --add-test-case and
--remove-test-case change it incrementally without restating the rest. The two forms
cannot be combined. At least one change is required.`,
		Example: `  saucectl authoring testsuites update 3f2a… --name "Checkout Regression v2"
  saucectl authoring testsuites update 3f2a… --add-test-case 6a882c1dc8b4482c166e96c9
  saucectl authoring testsuites update 3f2a… --remove-test-case 6a6b903c0405fb400076b2ba`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			s, err := testSuiteService.UpdateTestSuite(cmd.Context(), args[0], opts)
			if err != nil {
				return fmt.Errorf("failed to update test suite: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(s)
			}
			// testCaseCount in mutation responses lags behind the change (observed
			// 2026-09-06), so it is deliberately not quoted here.
			fmt.Printf("Updated test suite %q (%s).\n", s.Name, s.ID)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&opts.Name, "name", "", "New name.")
	flags.StringArrayVar(&opts.Tags, "tag", nil, "Replace the tags with these. Repeatable.")
	flags.StringArrayVar(&opts.TestCases, "test-case", nil, "Replace the membership with these test case IDs. Repeatable.")
	flags.StringArrayVar(&opts.AddTestCases, "add-test-case", nil, "Add this test case ID. Repeatable.")
	flags.StringArrayVar(&opts.RemoveTestCases, "remove-test-case", nil, "Remove this test case ID. Repeatable.")

	return cmd
}
