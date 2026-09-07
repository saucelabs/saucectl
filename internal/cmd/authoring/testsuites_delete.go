package authoring

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestSuitesDeleteCommand is `authoring testsuites delete`.
func TestSuitesDeleteCommand() *cobra.Command {
	var yes, deleteTestCases bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Delete a test suite",
		Long: `Delete a test suite. Its test cases are kept and become unassigned unless
--delete-test-cases is given, in which case every test case in the suite is deleted too.`,
		Example: `  saucectl authoring testsuites delete 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b
  saucectl authoring testsuites delete 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b --delete-test-cases --yes`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteTestSuite(cmd.Context(), args[0], yes, deleteTestCases)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt. Required when not running interactively.")
	flags.BoolVar(&deleteTestCases, "delete-test-cases", false, "Also delete every test case in the suite.")

	return cmd
}

// deleteTestSuite confirms, naming the suite's test cases and any schedules
// that trigger it, then deletes.
func deleteTestSuite(ctx context.Context, id string, yes, deleteTestCases bool) error {
	s, err := testSuiteService.GetTestSuite(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get test suite: %w", err)
	}

	// Only built when a prompt will actually be printed; see testcases_delete.go.
	var affects []string
	if yes {
		affects = nil
	} else if deleteTestCases {
		affects = append(affects, fmt.Sprintf("%d test case(s) in the suite will be DELETED", s.TestCaseCount))
	} else {
		affects = append(affects, fmt.Sprintf("%d test case(s) in the suite will be kept and become unassigned", s.TestCaseCount))
	}
	limit := 20
	schedules := authoring.List[authoring.TestSchedule]{}
	if !yes {
		schedules, err = scheduleService.ListSchedules(ctx, authoring.ListSchedulesOptions{ListOptions: authoring.ListOptions{Limit: &limit}, TestSuiteIDs: []string{s.ID}})
	}
	if !yes && err == nil && schedules.Total > 0 {
		for _, sch := range schedules.Items {
			affects = append(affects, fmt.Sprintf("schedule %q (%s) triggers this suite", sch.Name, sch.ID))
		}
		if schedules.Total > len(schedules.Items) {
			affects = append(affects, fmt.Sprintf("...and %d more schedule(s)", schedules.Total-len(schedules.Items)))
		}
	}

	if err := confirmDestructive(yes, fmt.Sprintf("test suite %q (%s)", s.Name, s.ID), affects); err != nil {
		return err
	}

	if err := testSuiteService.DeleteTestSuite(ctx, s.ID, deleteTestCases); err != nil {
		return fmt.Errorf("failed to delete test suite: %w", err)
	}
	fmt.Printf("Deleted test suite %q (%s).\n", s.Name, s.ID)
	return nil
}

// TestSuitesRunCommand is `authoring testsuites run`. It is fire-and-forget by
// contract: the service returns a queued count and the build name only, with
// no per-case run identifiers to follow (research R-002). Use `saucectl run`
// with a `kind: authoring` configuration to wait for results.
func TestSuitesRunCommand() *cobra.Command {
	var out, build string

	cmd := &cobra.Command{
		Use:   "run <id>",
		Short: "Queue a run of every test case in a suite (fire-and-forget)",
		Long: `Queue a run of every test case in the suite. The service reports only how many runs were
queued; it returns no run identifiers, so results cannot be followed from here. To wait
for results and gate a pipeline, use 'saucectl run' with a 'kind: authoring' configuration.`,
		Example:      `  saucectl authoring testsuites run 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b --build nightly`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			r, err := testSuiteService.RunTestSuite(cmd.Context(), args[0], build)
			if err != nil {
				return fmt.Errorf("failed to run test suite: %w", err)
			}
			if out == JSONOutput {
				return renderJSON(r)
			}
			fmt.Printf("Queued %d run(s) for test suite %s under build %q.\n", r.RunCount, args[0], r.BuildName)
			fmt.Println("Results are not followed here; see the Sauce Labs dashboard, or use 'saucectl run' with kind: authoring to wait for them.")
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&build, "build", "", "Build name to group the runs under.")

	return cmd
}
