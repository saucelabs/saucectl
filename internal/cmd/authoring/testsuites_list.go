package authoring

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestSuitesListCommand is `authoring testsuites list`.
func TestSuitesListCommand() *cobra.Command {
	var out string
	var page pageFlags
	var opts authoring.ListTestSuitesOptions

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List test suites",
		Example: `  saucectl authoring testsuites list
  saucectl authoring testsuites list --search regression -o json`,
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
			items, total, err := fetchPage(cmd.Context(), page, "test suites", func(ctx context.Context, lo authoring.ListOptions) (authoring.List[authoring.TestSuite], error) {
				opts.ListOptions = lo
				return testSuiteService.ListTestSuites(ctx, opts)
			})
			if err != nil {
				return fmt.Errorf("failed to list test suites: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(authoring.List[authoring.TestSuite]{Items: items, Total: total})
			}
			renderTestSuiteTable(items, total)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringArrayVar(&opts.IDs, "id", nil, "Only these suite IDs. Repeatable.")
	flags.StringVar(&opts.Search, "search", "", "Case-insensitive substring match on the name.")
	flags.StringVar(&opts.StartDate, "start-date", "", "Only suites created on or after this ISO 8601 date.")
	flags.StringVar(&opts.EndDate, "end-date", "", "Only suites created on or before this ISO 8601 date.")
	flags.StringVar(&opts.UserID, "user-id", "", "Filter by creator user ID.")
	flags.StringVar(&opts.TeamID, "team-id", "", "Filter by team ID.")
	page.bind(flags)

	return cmd
}

// renderTestSuiteTable prints the listing table, or a single line when empty.
func renderTestSuiteTable(items []authoring.TestSuite, total int) {
	if len(items) == 0 {
		fmt.Printf("No test suites found (total: %d).\n", total)
		return
	}
	t := newTable()
	t.AppendHeader(table.Row{"ID", "Name", "Tags", "Test Cases", "Updated", "Modified By"})
	for _, s := range items {
		t.AppendRow(table.Row{s.ID, truncate(s.Name, 50), joinOrDash(s.Tags), s.TestCaseCount, humanizeDate(s.LastUpdate), orDash(s.LastModifierUserName)})
	}
	t.AppendFooter(listFooter(len(items), total, "test suites"))
	fmt.Println(t.Render())
}

// TestSuitesGetCommand is `authoring testsuites get`.
func TestSuitesGetCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "get <id>",
		Short:        "Show a test suite",
		Example:      `  saucectl authoring testsuites get 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			s, err := testSuiteService.GetTestSuite(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get test suite: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(s)
			}
			renderTestSuiteDetail(s)
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}

// renderTestSuiteDetail prints the two-column property view.
func renderTestSuiteDetail(s authoring.TestSuite) {
	t := newTable()
	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRow(table.Row{"ID", s.ID})
	t.AppendRow(table.Row{"Name", s.Name})
	t.AppendRow(table.Row{"Tags", joinOrDash(s.Tags)})
	t.AppendRow(table.Row{"Test Cases", s.TestCaseCount})
	t.AppendRow(table.Row{"Team", orDash(s.TeamID)})
	t.AppendRow(table.Row{"Created", fmt.Sprintf("%s by %s", humanizeDate(s.CreationDate), orDash(s.CreatorUserName))})
	t.AppendRow(table.Row{"Updated", fmt.Sprintf("%s by %s", humanizeDate(s.LastUpdate), orDash(s.LastModifierUserName))})
	fmt.Println(t.Render())
	fmt.Printf("List its test cases with: saucectl authoring testcases list --test-suite-id %s\n", s.ID)
}
