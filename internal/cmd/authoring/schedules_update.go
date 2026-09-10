package authoring

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// unsettableFields are the settings --unset accepts. Each is cleared by
// sending an explicit null: omitting a field keeps its stored value (observed
// 2026-09-06, research Open-4).
var unsettableFields = []string{"tunnelName", "buildName", "startDate", "endDate", "maxRuns"}

// unsetConflicts maps each unsettable field to the flag that sets it, so
// asking for both in one command can be refused rather than silently
// resolved in favour of whichever runs last.
var unsetConflicts = map[string]string{
	"tunnelName": "tunnel-name",
	"buildName":  "build",
	"startDate":  "start-date",
	"endDate":    "end-date",
	"maxRuns":    "max-runs",
}

// scheduleUpdateFlags are the flags of `schedules update`.
type scheduleUpdateFlags struct {
	scheduleFlags
	addTestSuiteIDs    []string
	removeTestSuiteIDs []string
	unset              []string
}

// SchedulesUpdateCommand is `authoring schedules update`.
func SchedulesUpdateCommand() *cobra.Command {
	var out string
	var f scheduleUpdateFlags

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a test schedule",
		Long: `Update a test schedule. Only the given flags change; the schedule is read first and its
current settings, suites and state are re-sent in full, because the service replaces the
schedule rather than merging changes. An update therefore never erases what it does not
mention.

--unset clears an optional setting explicitly: tunnelName, buildName, startDate, endDate or
maxRuns. --test-suite-id replaces the suites wholesale; --add-test-suite-id and
--remove-test-suite-id change them incrementally.`,
		Example: `  saucectl authoring schedules update 9c1d… --cron "0 0 3 * * *"
  saucectl authoring schedules update 9c1d… --add-test-suite-id 3f2a… --unset maxRuns
  saucectl authoring schedules update 9c1d… --unset tunnelName`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			current, err := scheduleService.GetSchedule(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get schedule: %w", err)
			}
			opts, err := buildUpdateScheduleOptions(cmd.Flags(), f, current)
			if err != nil {
				return err
			}
			s, err := scheduleService.UpdateSchedule(cmd.Context(), args[0], opts)
			if err != nil {
				return fmt.Errorf("failed to update schedule: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(s)
			}
			fmt.Printf("Updated schedule %q (%s), %s, next run %s.\n", s.Name, s.ID, s.State.StateName, orDash(humanizeDate(s.State.NextRunDate)))
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	cmd.Flags().StringArrayVar(&f.testSuiteIDs, "test-suite-id", nil, "Replace the suites with these. Repeatable.")
	cmd.Flags().StringArrayVar(&f.addTestSuiteIDs, "add-test-suite-id", nil, "Add this suite. Repeatable.")
	cmd.Flags().StringArrayVar(&f.removeTestSuiteIDs, "remove-test-suite-id", nil, "Remove this suite. Repeatable.")
	cmd.Flags().StringArrayVar(&f.unset, "unset", nil, "Clear a setting: "+strings.Join(unsettableFields, ", ")+". Repeatable.")
	bindScheduleSettingsFlags(cmd, &f.scheduleFlags)

	return cmd
}

// noFlagChanges reports no flag as changed; used by enable/disable, which
// change only the state.
type noFlagChanges struct{}

// Changed always returns false.
func (noFlagChanges) Changed(string) bool { return false }

// buildUpdateScheduleOptions overlays the changed flags on the current
// schedule and produces the complete object the service requires.
//
// Observed 2026-09-06 (research Open-4, resolved): a partial body such as
// {"settings":{"cron":"…"}} is rejected with INVALID_BODY naming every
// missing required field — name, settings.timezone, settings.runningUserId,
// testSuiteIds and stateName — while omitted *optional* fields keep their
// stored values and an explicit null clears them. So the name, a settings
// object carrying the required fields, the full suite list and the state are
// always sent, --unset sends null, and --add-test-suite-id /
// --remove-test-suite-id are applied here rather than delegated. changed reports
// which flags the user set; it is the flag set in production and a stub in
// tests.
func buildUpdateScheduleOptions(changed interface{ Changed(string) bool }, f scheduleUpdateFlags, current authoring.TestSchedule) (authoring.UpdateScheduleOptions, error) {
	var opts authoring.UpdateScheduleOptions

	if f.testSuiteIDs != nil && (f.addTestSuiteIDs != nil || f.removeTestSuiteIDs != nil) {
		return opts, errors.New("--test-suite-id cannot be combined with --add-test-suite-id or --remove-test-suite-id")
	}

	settings := authoring.PatchFromSettings(current.Settings)
	touched := false
	touch := func() { touched = true }

	if changed.Changed("cron") {
		settings.Cron = f.cron
		touch()
	}
	if changed.Changed("timezone") {
		settings.Timezone = f.timezone
		touch()
	}
	if changed.Changed("running-user-id") {
		settings.RunningUserID = f.runningUserID
		touch()
	}
	if changed.Changed("start-date") {
		v := f.startDate
		settings.StartDate = &v
		touch()
	}
	if changed.Changed("end-date") {
		v := f.endDate
		settings.EndDate = &v
		touch()
	}
	if changed.Changed("max-runs") {
		if f.maxRuns < 0 {
			return opts, errors.New("--max-runs must not be negative")
		}
		m := f.maxRuns
		settings.MaxRuns = &m
		settings.ClearMaxRuns = false
		touch()
	}
	if changed.Changed("tunnel-name") {
		v := f.tunnelName
		settings.TunnelName = &v
		touch()
	}
	if changed.Changed("build") {
		v := f.build
		settings.BuildName = &v
		touch()
	}

	// Setting and clearing the same field in one invocation is ambiguous.
	// The unset loop runs last, so it used to win silently; say so instead.
	for _, field := range f.unset {
		if flag, ok := unsetConflicts[field]; ok && changed.Changed(flag) {
			return opts, fmt.Errorf("--%s and --unset %s conflict: pick one", flag, field)
		}
	}

	for _, field := range f.unset {
		empty := ""
		switch field {
		case "tunnelName":
			settings.TunnelName = &empty
		case "buildName":
			settings.BuildName = &empty
		case "startDate":
			settings.StartDate = &empty
		case "endDate":
			settings.EndDate = &empty
		case "maxRuns":
			settings.MaxRuns = nil
			settings.ClearMaxRuns = true
		default:
			return opts, fmt.Errorf("cannot unset %q; options: %s", field, strings.Join(unsettableFields, ", "))
		}
		touch()
	}

	name := current.Name
	if changed.Changed("name") {
		name = f.name
		touch()
	}

	state := current.State.StateName
	if f.state != "" {
		parsed, ok := authoring.ParseScheduleState(f.state)
		if !ok {
			return opts, fmt.Errorf("invalid --state %q; options: ENABLED, DISABLED", f.state)
		}
		state = parsed
		touch()
	} else if _, settable := authoring.ParseScheduleState(string(state)); !settable {
		// RUNNING and ERRORED are observed states the service will not accept
		// back. Rather than guess, make the user choose.
		return opts, fmt.Errorf("schedule is currently %s; pass --state ENABLED or --state DISABLED to update it", state)
	}

	suites := append([]string(nil), current.TestSuiteIDs...)
	if f.testSuiteIDs != nil {
		suites = append([]string(nil), f.testSuiteIDs...)
		touch()
	}
	for _, id := range f.addTestSuiteIDs {
		if !contains(suites, id) {
			suites = append(suites, id)
		}
		touch()
	}
	for _, id := range f.removeTestSuiteIDs {
		kept := suites[:0]
		for _, s := range suites {
			if s != id {
				kept = append(kept, s)
			}
		}
		suites = kept
		touch()
	}
	if len(suites) == 0 {
		return opts, errors.New("a schedule must keep at least one test suite")
	}

	if !touched {
		return opts, authoring.ErrEmptyUpdate
	}

	return authoring.UpdateScheduleOptions{
		Name:         name,
		Settings:     &settings,
		TestSuiteIDs: suites,
		StateName:    state,
	}, nil
}

// Compile-time assertion that a pflag.FlagSet satisfies the Changed interface
// buildUpdateScheduleOptions takes.
var _ interface{ Changed(string) bool } = (*pflag.FlagSet)(nil)
