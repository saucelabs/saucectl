package authoring

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestCasesDeleteCommand is `authoring testcases delete`.
func TestCasesDeleteCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:          "delete <id>",
		Aliases:      []string{"rm"},
		Short:        "Delete a test case",
		Example:      `  saucectl authoring testcases delete 6a882c1dc8b4482c166e96c9 --yes`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteTestCase(cmd.Context(), args[0], yes)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt. Required when not running interactively.")

	return cmd
}

// deleteTestCase confirms, then deletes. The confirmation names the case and
// what depends on it (FR-038): its suite membership, its revisions and the
// run history that will be orphaned.
func deleteTestCase(ctx context.Context, id string, yes bool) error {
	tc, err := testCaseService.GetTestCase(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get test case: %w", err)
	}

	// These lookups exist only to fill the prompt. With --yes there is no
	// prompt, so issuing them would add a wasted round trip per asset on the
	// scripted path, on top of the entitlement gate's two.
	var affects []string
	if !yes && tc.TestSuiteID != "" {
		affects = append(affects, fmt.Sprintf("it will be removed from suite %s", tc.TestSuiteID))
	}
	// Observed 2026-09-06: run history outlives the test case. Runs of a
	// deleted case still come back from the filtered list and from the run
	// detail endpoint, so they are not lost — only orphaned.
	zero := 0
	runs := authoring.List[authoring.Run]{}
	if !yes {
		runs, err = testCaseService.ListRuns(ctx, tc.ID, authoring.ListRunsOptions{ListOptions: authoring.ListOptions{Limit: &zero}})
	}
	if !yes && err == nil && runs.Total > 0 {
		affects = append(affects, fmt.Sprintf("its %d recorded run(s) stay in run history but will belong to a test case that no longer exists", runs.Total))
	}
	if !yes && len(tc.Revisions) > 1 {
		affects = append(affects, fmt.Sprintf("all %d revisions are deleted", len(tc.Revisions)))
	}

	if err := confirmDestructive(yes, fmt.Sprintf("test case %q (%s)", tc.Name, tc.ID), affects); err != nil {
		return err
	}

	if err := testCaseService.DeleteTestCase(ctx, tc.ID); err != nil {
		return fmt.Errorf("failed to delete test case: %w", err)
	}
	fmt.Printf("Deleted test case %q (%s).\n", tc.Name, tc.ID)
	return nil
}
