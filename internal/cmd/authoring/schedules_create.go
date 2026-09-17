package authoring

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// scheduleFlags are the settings flags shared by create and update.
type scheduleFlags struct {
	name          string
	cron          string
	timezone      string
	runningUserID string
	testSuiteIDs  []string
	state         string
	startDate     string
	endDate       string
	maxRuns       int
	tunnelName    string
	build         string
}

// bindScheduleSettingsFlags registers the settings flags. Defaults are left
// empty so update can tell an unset flag from an explicit one via Changed.
func bindScheduleSettingsFlags(cmd *cobra.Command, f *scheduleFlags) {
	flags := cmd.Flags()
	flags.StringVar(&f.name, "name", "", "Name of the schedule (1–255 characters).")
	flags.StringVar(&f.cron, "cron", "", "Six-field cron expression starting with seconds, e.g. \"0 0 6 * * 1-5\" for 06:00 on weekdays.")
	flags.StringVar(&f.timezone, "timezone", "", "IANA region/city timezone for the cron expression, e.g. Europe/Berlin or America/New_York. Required on create. The service does not accept \"UTC\"; use a zero-offset zone such as Atlantic/Reykjavik instead.")
	flags.StringVar(&f.runningUserID, "running-user-id", "", "User the scheduled runs execute as. Default: you.")
	flags.StringVar(&f.state, "state", "", "ENABLED or DISABLED (case-insensitive). Default: ENABLED.")
	flags.StringVar(&f.startDate, "start-date", "", "ISO 8601 date before which the schedule does not run.")
	flags.StringVar(&f.endDate, "end-date", "", "ISO 8601 date after which the schedule stops.")
	flags.IntVar(&f.maxRuns, "max-runs", 0, "Maximum number of runs before the schedule disables itself.")
	flags.StringVar(&f.tunnelName, "tunnel-name", "", "Sauce Connect tunnel to route scheduled runs through.")
	flags.StringVar(&f.build, "build", "", "Build name for scheduled runs.")

	_ = cmd.RegisterFlagCompletionFunc("state", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		names := make([]string, len(authoring.SettableScheduleStates))
		for i, s := range authoring.SettableScheduleStates {
			names[i] = string(s)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})
}

// SchedulesCreateCommand is `authoring schedules create`.
func SchedulesCreateCommand() *cobra.Command {
	var out string
	var f scheduleFlags

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a test schedule",
		Example: `  saucectl authoring schedules create --name "Nightly" --cron "0 0 2 * * *" --timezone Europe/Berlin \
      --test-suite-id 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b --build nightly
  saucectl authoring schedules create --name "Smoke" --cron "0 */30 * * * *" --timezone America/New_York --test-suite-id 3f2a… --max-runs 10 --state disabled`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			opts, err := buildCreateScheduleOptions(f, currentUser.ID)
			if err != nil {
				return err
			}
			s, err := scheduleService.CreateSchedule(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("failed to create schedule: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(s)
			}
			fmt.Printf("Created schedule %q (%s), %s, next run %s.\n", s.Name, s.ID, s.State.StateName, orDash(humanizeDate(s.State.NextRunDate)))
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	cmd.Flags().StringArrayVar(&f.testSuiteIDs, "test-suite-id", nil, "Suite to run. Repeatable; at least one is required.")
	bindScheduleSettingsFlags(cmd, &f)

	return cmd
}

// buildCreateScheduleOptions validates and assembles the create request,
// applying the defaults: running user = the caller, state ENABLED.
func buildCreateScheduleOptions(f scheduleFlags, callerID string) (authoring.CreateScheduleOptions, error) {
	var opts authoring.CreateScheduleOptions
	if f.name == "" {
		return opts, errors.New("--name is required")
	}
	if f.cron == "" {
		return opts, errors.New("--cron is required")
	}
	if len(f.testSuiteIDs) == 0 {
		return opts, errors.New("at least one --test-suite-id is required")
	}
	// Observed 2026-09-06: the service validates the timezone against an IANA
	// region/city list that contains neither "UTC" nor "Etc/UTC", so there is
	// no safe default to fall back to.
	if f.timezone == "" {
		return opts, errors.New("--timezone is required: an IANA region/city zone such as Europe/Berlin (the service does not accept \"UTC\")")
	}
	// Observed 2026-09-06: the service's accepted list is region/city zones
	// only and contained neither of these, so catch them here rather than
	// spending a round trip on an INVALID_BODY. Anything else is left to the
	// service, which holds the real list; a local tzdata check would not help
	// because Go itself accepts "UTC".
	switch strings.ToLower(f.timezone) {
	case "utc", "etc/utc":
		return opts, fmt.Errorf("--timezone %q is not accepted by the service; use an IANA region/city zone, e.g. Atlantic/Reykjavik for a zero offset", f.timezone)
	}
	if f.maxRuns < 0 {
		return opts, errors.New("--max-runs must not be negative")
	}

	state := authoring.ScheduleEnabled
	if f.state != "" {
		parsed, ok := authoring.ParseScheduleState(f.state)
		if !ok {
			return opts, fmt.Errorf("invalid --state %q; options: ENABLED, DISABLED", f.state)
		}
		state = parsed
	}

	runner := f.runningUserID
	if runner == "" {
		runner = callerID
	}
	if runner == "" {
		return opts, errors.New("--running-user-id is required: the current user could not be resolved")
	}

	settings := authoring.ScheduleSettings{
		Cron:          f.cron,
		Timezone:      f.timezone,
		RunningUserID: runner,
		StartDate:     f.startDate,
		EndDate:       f.endDate,
		TunnelName:    f.tunnelName,
		BuildName:     f.build,
	}
	if f.maxRuns > 0 {
		m := f.maxRuns
		settings.MaxRuns = &m
	}

	return authoring.CreateScheduleOptions{
		Name:         f.name,
		Settings:     settings,
		TestSuiteIDs: f.testSuiteIDs,
		StateName:    state,
	}, nil
}
