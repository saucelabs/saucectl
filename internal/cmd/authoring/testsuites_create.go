package authoring

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestSuitesCreateCommand is `authoring testsuites create`.
func TestSuitesCreateCommand() *cobra.Command {
	var out string
	var opts authoring.CreateTestSuiteOptions

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a test suite",
		Example: `  saucectl authoring testsuites create --name "Checkout Regression" --tag smoke \
      --test-case 6a882c1dc8b4482c166e96c9 --test-case 6a6b903c0405fb400076b2ba`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			if opts.Name == "" {
				return errors.New("--name is required")
			}
			s, err := testSuiteService.CreateTestSuite(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("failed to create test suite: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(s)
			}
			// testCaseCount in mutation responses lags behind the change (observed
			// 2026-09-06), so it is deliberately not quoted here.
			fmt.Printf("Created test suite %q (%s).\n", s.Name, s.ID)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&opts.Name, "name", "", "Name of the suite (1–255 characters). Required.")
	flags.StringArrayVar(&opts.Tags, "tag", nil, "Tag for the suite. Repeatable.")
	flags.StringArrayVar(&opts.TestCases, "test-case", nil, "Test case ID to include. Repeatable.")

	return cmd
}
