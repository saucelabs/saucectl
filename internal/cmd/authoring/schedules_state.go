package authoring

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// SchedulesEnableCommand is `authoring schedules enable`, a convenience over
// update that sets the state to ENABLED.
func SchedulesEnableCommand() *cobra.Command {
	return scheduleStateCommand("enable", "Resume a suspended schedule", authoring.ScheduleEnabled)
}

// SchedulesDisableCommand is `authoring schedules disable`, a convenience over
// update that sets the state to DISABLED. No runs are triggered while disabled.
func SchedulesDisableCommand() *cobra.Command {
	return scheduleStateCommand("disable", "Suspend a schedule without deleting it", authoring.ScheduleDisabled)
}

// scheduleStateCommand builds enable/disable.
func scheduleStateCommand(use, short string, state authoring.ScheduleStateName) *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:          use + " <id>",
		Short:        short,
		Example:      fmt.Sprintf("  saucectl authoring schedules %s 9c1d…", use),
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			// The update endpoint replaces the whole schedule, so even a
			// state change is a read-modify-write of the complete object.
			current, err := scheduleService.GetSchedule(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get schedule: %w", err)
			}
			opts, err := buildUpdateScheduleOptions(noFlagChanges{}, scheduleUpdateFlags{scheduleFlags: scheduleFlags{state: string(state)}}, current)
			if err != nil {
				return err
			}
			s, err := scheduleService.UpdateSchedule(cmd.Context(), args[0], opts)
			if err != nil {
				return fmt.Errorf("failed to %s schedule: %w", use, err)
			}
			if out == JSONOutput {
				return renderJSON(s)
			}
			fmt.Printf("Schedule %q (%s) is now %s.\n", s.Name, s.ID, s.State.StateName)
			return nil
		},
	}

	cmd.Flags().StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")

	return cmd
}

// SchedulesDeleteCommand is `authoring schedules delete`.
func SchedulesDeleteCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:          "delete <id>",
		Aliases:      []string{"rm"},
		Short:        "Delete a test schedule",
		Example:      `  saucectl authoring schedules delete 9c1d… --yes`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteSchedule(cmd.Context(), args[0], yes)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt. Required when not running interactively.")

	return cmd
}

// deleteSchedule confirms, naming the schedule's state, cadence and suites,
// then deletes.
func deleteSchedule(ctx context.Context, id string, yes bool) error {
	s, err := scheduleService.GetSchedule(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get schedule: %w", err)
	}

	affects := []string{
		fmt.Sprintf("it is %s and runs %q (%s)", s.State.StateName, s.Settings.Cron, s.Settings.Timezone),
		fmt.Sprintf("it triggers %d suite(s): %s", len(s.TestSuiteIDs), joinOrDash(s.TestSuiteIDs)),
	}
	if s.State.NextRunDate != "" && s.State.StateName == authoring.ScheduleEnabled {
		affects = append(affects, fmt.Sprintf("the next run at %s will not happen", humanizeDate(s.State.NextRunDate)))
	}

	if err := confirmDestructive(yes, fmt.Sprintf("schedule %q (%s)", s.Name, s.ID), affects); err != nil {
		return err
	}

	if err := scheduleService.DeleteSchedule(ctx, s.ID); err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}
	fmt.Printf("Deleted schedule %q (%s).\n", s.Name, s.ID)
	return nil
}
