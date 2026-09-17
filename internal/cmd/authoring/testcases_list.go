package authoring

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestCasesListCommand is `authoring testcases list`.
func TestCasesListCommand() *cobra.Command {
	var out string
	var page pageFlags
	var opts authoring.ListTestCasesOptions

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List test cases",
		Example: `  saucectl authoring testcases list
  saucectl authoring testcases list --search checkout --tag smoke
  saucectl authoring testcases list --test-suite-id 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b -o json
  saucectl authoring testcases list --limit 0        # total count only`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			if err := page.validate(); err != nil {
				return err
			}
			page.capture(cmd.Flags())
			return listTestCases(cmd.Context(), out, page, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&opts.Search, "search", "", "Case-insensitive substring match on the name.")
	flags.StringVar(&opts.StartDate, "start-date", "", "Only test cases created on or after this ISO 8601 date.")
	flags.StringVar(&opts.EndDate, "end-date", "", "Only test cases created on or before this ISO 8601 date.")
	flags.StringVar(&opts.UserID, "user-id", "", "Filter by creator user ID.")
	flags.StringVar(&opts.TeamID, "team-id", "", "Filter by team ID.")
	flags.StringArrayVar(&opts.TestSuiteIDs, "test-suite-id", nil, "Filter by test suite ID. Repeatable. Use 'null' for test cases in no suite.")
	flags.StringArrayVar(&opts.Tags, "tag", nil, "Filter by tag (case-sensitive). Repeatable; matches test cases with any of the tags.")
	page.bind(flags)

	return cmd
}

// listTestCases fetches and renders the listing.
func listTestCases(ctx context.Context, out string, page pageFlags, opts authoring.ListTestCasesOptions) error {
	items, total, err := fetchPage(ctx, page, "test cases", func(ctx context.Context, lo authoring.ListOptions) (authoring.List[authoring.TestCase], error) {
		opts.ListOptions = lo
		return testCaseService.ListTestCases(ctx, opts)
	})
	if err != nil {
		return fmt.Errorf("failed to list test cases: %w", err)
	}

	if out == JSONOutput {
		return renderJSON(authoring.List[authoring.TestCase]{Items: items, Total: total})
	}
	renderTestCaseTable(items, total)
	return nil
}

// renderTestCaseTable prints the listing table, or a single line when empty.
func renderTestCaseTable(items []authoring.TestCase, total int) {
	if len(items) == 0 {
		fmt.Printf("No test cases found (total: %d).\n", total)
		return
	}

	t := newTable()
	t.AppendHeader(table.Row{"ID", "Name", "Tags", "Suite", "Steps", "Updated", "Modified By"})
	for _, tc := range items {
		steps := "-"
		if rev, ok := tc.LatestRevision(); ok {
			steps = fmt.Sprint(len(rev.Steps))
		}
		t.AppendRow(table.Row{
			tc.ID,
			truncate(tc.Name, 50),
			joinOrDash(tc.Tags),
			orDash(tc.TestSuiteID),
			steps,
			humanizeDate(tc.LastUpdateDate),
			orDash(tc.LastModifierUserName),
		})
	}
	t.AppendFooter(listFooter(len(items), total, "test cases"))
	fmt.Println(t.Render())
}
