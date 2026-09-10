package authoring

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// TestCasesRunCommand is `authoring testcases run`. It starts a run and
// returns immediately with the run's identifiers; use `list-runs` and
// `get-run` to follow it, or `saucectl run` with a `kind: authoring`
// configuration to wait, report and gate a pipeline.
func TestCasesRunCommand() *cobra.Command {
	var out string
	var build, tunnelName, revisionID string
	var kvTargets, jsonTargets []string

	cmd := &cobra.Command{
		Use:   "run <id>",
		Short: "Start a run of a test case",
		Long: `Start a run of a test case against its stored run targets, or against the targets given
with --target / --target-json. The command returns as soon as the run is accepted; use
'testcases get-run' to check on it. To wait for results, report them and fail a pipeline,
use 'saucectl run' with a 'kind: authoring' configuration instead.

Without --tunnel-name the run is started with no tunnel, which also clears any tunnel
name stored on the test case.`,
		Example: `  saucectl authoring testcases run 6a882c1dc8b4482c166e96c9 --build nightly
  saucectl authoring testcases run 6a882c1dc8b4482c166e96c9 --target browserName=firefox,platformName="Windows 11"
  saucectl authoring testcases run 6a882c1dc8b4482c166e96c9 --target-json @pixel9.json --tunnel-name my-tunnel`,
		SilenceUsage: true,
		Args:         requireArgs("id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(out); err != nil {
				return err
			}
			targets, err := parseTargets(kvTargets, jsonTargets)
			if err != nil {
				return err
			}
			return runTestCase(cmd.Context(), out, args[0], revisionID, authoring.RunOptions{
				BuildName:  build,
				TunnelName: tunnelName,
				Targets:    targets,
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", TextOutput, "Output format. Options: text, json.")
	flags.StringVar(&build, "build", "", "Build name to group the run's jobs under (max 100 characters).")
	flags.StringVar(&tunnelName, "tunnel-name", "", "Name of an active Sauce Connect tunnel to route the run through.")
	flags.StringArrayVar(&kvTargets, "target", nil, "Target capabilities as key=value pairs, e.g. browserName=chrome,platformName=\"Windows 11\". Repeatable.")
	flags.StringArrayVar(&jsonTargets, "target-json", nil, "Target capabilities as a JSON object, or @path to a file. Repeatable.")
	flags.StringVar(&revisionID, "revision", "", "Run this revision instead of the latest.")

	return cmd
}

// runTestCase starts the run and renders its identifiers.
func runTestCase(ctx context.Context, out, id, revisionID string, opts authoring.RunOptions) error {
	run, err := testCaseService.RunTestCase(ctx, id, revisionID, opts)
	if err != nil {
		return fmt.Errorf("failed to start run: %w", err)
	}

	if out == JSONOutput {
		return renderJSON(run)
	}

	fmt.Printf("Run %s started for test case %s (build %q).\n", run.ID, run.TestCaseID, run.Build)
	renderJobs(run.Jobs)
	fmt.Printf("Check on it with: saucectl authoring testcases get-run %s %s\n", run.TestCaseID, run.ID)
	return nil
}

// renderJobs prints one row per job with its derived dashboard link.
func renderJobs(jobs []authoring.RunJob) {
	if len(jobs) == 0 {
		fmt.Println("No jobs reported yet.")
		return
	}

	t := newTable()
	t.AppendHeader(table.Row{"Job", "Target", "Status", "Error", "URL"})
	for _, j := range jobs {
		t.AppendRow(table.Row{
			orDash(j.SauceJobID),
			describeTarget(j.Target),
			jobStatus(j),
			truncate(orDash(j.Error), 60),
			orDash(jobURL(j.SauceJobID)),
		})
	}
	fmt.Println(t.Render())
}

// jobStatus renders a job's inferred state: there is no status field on the
// wire (research R-005).
func jobStatus(j authoring.RunJob) string {
	switch {
	case j.Passed():
		return "passed"
	case j.Done():
		return "failed"
	default:
		return "in progress"
	}
}

// runStatus summarises a run's jobs as e.g. "2/3 passed" or "in progress".
func runStatus(r authoring.Run) string {
	if !r.Done() {
		return "in progress"
	}
	passed := 0
	for _, j := range r.Jobs {
		if j.Passed() {
			passed++
		}
	}
	if passed == len(r.Jobs) {
		return fmt.Sprintf("passed (%d/%d)", passed, len(r.Jobs))
	}
	return fmt.Sprintf("failed (%d/%d passed)", passed, len(r.Jobs))
}
