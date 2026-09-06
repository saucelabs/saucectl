package authoring

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestCasesListRunsCommand is `authoring testcases list-runs`.
func TestCasesListRunsCommand() *cobra.Command {
	var out string
	var page pageFlags
	var opts authoring.ListRunsOptions

	cmd := &cobra.Command{
		Use:     "list-runs <id>",
		Aliases: []string{"runs"},
		Short:   "List the runs of a test case",
		Example: `  saucectl authoring testcases list-runs 6a882c1dc8b4482c166e96c9
  saucectl authoring testcases list-runs 6a882c1dc8b4482c166e96c9 --all -o json | jq -r '[.items[].testCaseId] | unique'`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			if err := page.validate(); err != nil {
				return err
			}
			return listRuns(cmd.Context(), out, args[0], page, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&opts.StartDate, "start-date", "", "Only runs created on or after this ISO 8601 date.")
	flags.StringVar(&opts.EndDate, "end-date", "", "Only runs created on or before this ISO 8601 date.")
	flags.StringVar(&opts.UserID, "user-id", "", "Only runs started by this user ID.")
	flags.StringVar(&opts.TeamID, "team-id", "", "Only runs belonging to this team ID.")
	page.bind(flags)

	return cmd
}

// listRuns fetches and renders the runs of one test case. The service filters
// on the testCaseId query parameter, which ListRuns always sends; without it
// the whole organisation's runs would come back (research R-004).
func listRuns(ctx context.Context, out, testCaseID string, page pageFlags, opts authoring.ListRunsOptions) error {
	items, total, err := fetchPage(ctx, page, "runs", func(ctx context.Context, lo authoring.ListOptions) (authoring.List[authoring.Run], error) {
		opts.ListOptions = lo
		return testCaseService.ListRuns(ctx, testCaseID, opts)
	})
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	if out == JSONOutput {
		return renderJSON(authoring.List[authoring.Run]{Items: items, Total: total})
	}

	if len(items) == 0 {
		fmt.Printf("No runs found (total: %d).\n", total)
		return nil
	}
	t := newTable()
	t.AppendHeader(table.Row{"Run ID", "Build", "Jobs", "Status", "Created"})
	for _, r := range items {
		t.AppendRow(table.Row{r.ID, truncate(orDash(r.Build), 40), len(r.Jobs), runStatus(r), humanizeDate(r.CreationDate)})
	}
	t.AppendFooter(listFooter(len(items), total, "runs"))
	fmt.Println(t.Render())
	return nil
}

// TestCasesGetRunCommand is `authoring testcases get-run`.
func TestCasesGetRunCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "get-run <id> <run-id>",
		Short:        "Show a single run with its jobs",
		Example:      `  saucectl authoring testcases get-run 6a882c1dc8b4482c166e96c9 ed320d67-5f56-4e81-b61d-72e325661c77`,
		SilenceUsage: true,
		Args:         requireArgs("id", "run-id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			run, err := testCaseService.GetRun(cmd.Context(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("failed to get run: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(run)
			}
			renderRunDetail(run)
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}

// renderRunDetail prints a run's properties followed by its jobs.
func renderRunDetail(run authoring.Run) {
	t := newTable()
	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRow(table.Row{"Run ID", run.ID})
	t.AppendRow(table.Row{"Test Case", run.TestCaseID})
	t.AppendRow(table.Row{"Build", orDash(run.Build)})
	t.AppendRow(table.Row{"Status", runStatus(run)})
	t.AppendRow(table.Row{"Created", humanizeDate(run.CreationDate)})
	t.AppendRow(table.Row{"Test URL", orDash(run.TestURL)})
	fmt.Println(t.Render())
	renderJobs(run.Jobs)
}
