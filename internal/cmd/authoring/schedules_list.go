package authoring

import (
	"context"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// SchedulesListCommand is `authoring schedules list`.
func SchedulesListCommand() *cobra.Command {
	var out string
	var page pageFlags
	var opts authoring.ListSchedulesOptions

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List test schedules",
		Example: `  saucectl authoring schedules list
  saucectl authoring schedules list --test-suite-id 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b`,
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
			items, total, err := fetchPage(cmd.Context(), page, "schedules", func(ctx context.Context, lo authoring.ListOptions) (authoring.List[authoring.TestSchedule], error) {
				opts.ListOptions = lo
				return scheduleService.ListSchedules(ctx, opts)
			})
			if err != nil {
				return fmt.Errorf("failed to list schedules: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(authoring.List[authoring.TestSchedule]{Items: items, Total: total})
			}
			renderScheduleTable(items, total)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringArrayVar(&opts.IDs, "id", nil, "Only these schedule IDs. Repeatable.")
	flags.StringVar(&opts.Search, "search", "", "Case-insensitive substring match on the name.")
	flags.StringVar(&opts.StartDate, "start-date", "", "Only schedules created on or after this ISO 8601 date.")
	flags.StringVar(&opts.EndDate, "end-date", "", "Only schedules created on or before this ISO 8601 date.")
	flags.StringVar(&opts.UserID, "user-id", "", "Filter by creator user ID.")
	flags.StringVar(&opts.TeamID, "team-id", "", "Filter by team ID.")
	flags.StringArrayVar(&opts.TestSuiteIDs, "test-suite-id", nil, "Only schedules that trigger this suite. Repeatable.")
	page.bind(flags)

	return cmd
}

// renderScheduleTable prints the listing table, or a single line when empty.
func renderScheduleTable(items []authoring.TestSchedule, total int) {
	if len(items) == 0 {
		fmt.Printf("No schedules found (total: %d).\n", total)
		return
	}
	t := newTable()
	t.AppendHeader(table.Row{"ID", "Name", "State", "Cron", "Timezone", "Next Run", "Suites"})
	for _, s := range items {
		t.AppendRow(table.Row{s.ID, truncate(s.Name, 40), s.State.StateName, s.Settings.Cron, s.Settings.Timezone, orDash(humanizeDate(s.State.NextRunDate)), len(s.TestSuiteIDs)})
	}
	t.AppendFooter(listFooter(len(items), total, "schedules"))
	fmt.Println(t.Render())
}

// SchedulesGetCommand is `authoring schedules get`.
func SchedulesGetCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          "get <id>",
		Short:        "Show a test schedule",
		Example:      `  saucectl authoring schedules get 9c1d…`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			s, err := scheduleService.GetSchedule(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get schedule: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(s)
			}
			renderScheduleDetail(s)
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}

// renderScheduleDetail prints the two-column property view.
func renderScheduleDetail(s authoring.TestSchedule) {
	maxRuns := "unlimited"
	if s.Settings.MaxRuns != nil {
		maxRuns = fmt.Sprint(*s.Settings.MaxRuns)
	}
	remaining := "-"
	if s.State.RemainingRuns != nil {
		remaining = fmt.Sprint(*s.State.RemainingRuns)
	}

	t := newTable()
	t.AppendHeader(table.Row{"Property", "Value"})
	t.AppendRow(table.Row{"ID", s.ID})
	t.AppendRow(table.Row{"Name", s.Name})
	t.AppendRow(table.Row{"State", s.State.StateName})
	t.AppendRow(table.Row{"Cron", s.Settings.Cron})
	t.AppendRow(table.Row{"Timezone", s.Settings.Timezone})
	t.AppendRow(table.Row{"Running User", s.Settings.RunningUserID})
	t.AppendRow(table.Row{"Start Date", orDash(s.Settings.StartDate)})
	t.AppendRow(table.Row{"End Date", orDash(s.Settings.EndDate)})
	t.AppendRow(table.Row{"Max Runs", maxRuns})
	t.AppendRow(table.Row{"Remaining Runs", remaining})
	t.AppendRow(table.Row{"Tunnel", orDash(s.Settings.TunnelName)})
	t.AppendRow(table.Row{"Build", orDash(s.Settings.BuildName)})
	t.AppendRow(table.Row{"Suites", strings.Join(s.TestSuiteIDs, "\n")})
	t.AppendRow(table.Row{"Last Run", orDash(humanizeDate(s.State.LastRunDate))})
	t.AppendRow(table.Row{"Last Run Error", orDash(s.State.LastRunError)})
	t.AppendRow(table.Row{"Next Run", orDash(humanizeDate(s.State.NextRunDate))})
	t.AppendRow(table.Row{"Created", fmt.Sprintf("%s by %s", humanizeDate(s.CreationDate), orDash(s.CreatorUserName))})
	t.AppendRow(table.Row{"Updated", fmt.Sprintf("%s by %s", humanizeDate(s.LastUpdateDate), orDash(s.LastModifierUserName))})
	fmt.Println(t.Render())
}
