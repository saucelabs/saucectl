package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/aiauthoring"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/tables"
	"github.com/spf13/cobra"
)

// runPollInterval matches the polling cadence of saucectl run.
const runPollInterval = 15 * time.Second

// testCaseIDLength is the length of a test case id (a hex object id), while
// test suite ids are 32 hex characters. Used to give a helpful hint when an
// id is passed to the wrong command.
const (
	testCaseIDLength  = 24
	testSuiteIDLength = 32
)

// runResult is the final state of one job of a test case run.
type runResult struct {
	Name       string `json:"name"`
	SauceJobID string `json:"sauceJobId,omitempty"`
	Status     string `json:"status"`
	Passed     bool   `json:"passed"`
	URL        string `json:"url,omitempty"`
	Error      string `json:"error,omitempty"`
}

// runOptions are the settings shared by test case and suite runs.
type runOptions struct {
	Targets   []aiauthoring.RunTarget
	Build     string
	Tunnel    string
	Async     bool
	Timeout   time.Duration
	OutFormat string
}

// runFlags are the flags shared by 'ai run' and 'ai run-suite'.
type runFlags struct {
	out            string
	build          string
	tunnelName     string
	browser        string
	browserVersion string
	platform       string
	async          bool
	timeout        time.Duration
}

func (f *runFlags) register(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringVarP(&f.out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.StringVar(&f.build, "build", "", "Associates the run with a build name.")
	flags.StringVar(&f.tunnelName, "tunnel-name", "", "Runs the test through a Sauce Connect tunnel with the given name.")
	flags.StringVar(&f.browser, "browser", "", "The browser to run against, e.g. chrome. Overrides the authored target.")
	flags.StringVar(&f.browserVersion, "browser-version", "", "The browser version to run against, e.g. latest.")
	flags.StringVar(&f.platform, "platform", "", "The platform to run against, e.g. \"Windows 11\". Overrides the authored target.")
	flags.BoolVar(&f.async, "async", false, "Starts the run without waiting for the result.")
	flags.DurationVar(&f.timeout, "timeout", 30*time.Minute, "The maximum time to wait for the run to finish.")
}

func (f *runFlags) toOptions() (runOptions, error) {
	if f.out != JSONOutput && f.out != TextOutput {
		return runOptions{}, errors.New("unknown output format")
	}

	var targets []aiauthoring.RunTarget
	if f.browser != "" || f.platform != "" {
		caps := aiauthoring.RunCapabilities{}
		if f.browser != "" {
			caps["browserName"] = f.browser
		}
		if f.browserVersion != "" {
			caps["browserVersion"] = f.browserVersion
		}
		if f.platform != "" {
			caps["platformName"] = f.platform
		}
		targets = []aiauthoring.RunTarget{{Capabilities: caps}}
	}

	return runOptions{
		Targets:   targets,
		Build:     f.build,
		Tunnel:    f.tunnelName,
		Async:     f.async,
		Timeout:   f.timeout,
		OutFormat: f.out,
	}, nil
}

func RunCommand() *cobra.Command {
	var flags runFlags

	cmd := &cobra.Command{
		Use:   "run <testCaseID>",
		Short: "Runs a saved test case in the Sauce Labs cloud",
		Long: `Runs a saved AI-authored test case in the Sauce Labs cloud and waits for the
result, like 'saucectl run' does for framework tests. To run a whole test
suite, use 'saucectl ai run-suite'.

The target platform is taken from --browser/--platform when given, otherwise
from the test case's authored run settings, otherwise the backend picks its
default. Use --async to start the run without waiting for it to finish.`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return errors.New("no test case ID specified")
			}
			if len(args[0]) == testSuiteIDLength {
				return fmt.Errorf("%q looks like a test suite ID; use `saucectl ai run-suite` to run a test suite", args[0])
			}
			return nil
		},
		PreRun: collectUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := flags.toOptions()
			if err != nil {
				return err
			}
			return runTestCase(cmd.Context(), args[0], opts)
		},
	}

	flags.register(cmd)

	return cmd
}

func RunSuiteCommand() *cobra.Command {
	var flags runFlags

	cmd := &cobra.Command{
		Use:   "run-suite <testSuiteID>",
		Short: "Runs all test cases of a saved test suite in the Sauce Labs cloud",
		Long: `Runs every test case of a saved AI-authored test suite in the Sauce Labs
cloud and aggregates the results. The test cases run concurrently, each as
its own Sauce Labs job.

The target platform is taken from --browser/--platform when given, otherwise
from each test case's authored run settings, otherwise the backend picks its
default. Use --async to start the runs without waiting for the results.`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] == "" {
				return errors.New("no test suite ID specified")
			}
			if len(args[0]) == testCaseIDLength {
				return fmt.Errorf("%q looks like a test case ID; use `saucectl ai run` to run a single test case", args[0])
			}
			return nil
		},
		PreRun: collectUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := flags.toOptions()
			if err != nil {
				return err
			}
			return runSuite(cmd.Context(), args[0], opts)
		},
	}

	flags.register(cmd)

	return cmd
}

func runTestCase(ctx context.Context, id string, opts runOptions) error {
	tc, err := testCaseService.GetTestCase(ctx, id)
	if err != nil {
		return err
	}

	run, err := startRun(ctx, tc, opts)
	if err != nil {
		return err
	}

	if opts.OutFormat == TextOutput {
		fmt.Printf("Started run %s of test case %s with %d job(s)\n", run.ID, run.TestCaseID, len(run.Jobs))
		if run.TestURL != "" {
			fmt.Printf("Build URL: %s\n", run.TestURL)
		}
	}

	if opts.Async {
		if opts.OutFormat == JSONOutput {
			return renderJSON(run)
		}
		return nil
	}

	final, err := pollRun(ctx, id, run, opts.Timeout)
	if err != nil {
		return err
	}

	return reportResults(toRunResults(final), opts.OutFormat)
}

func runSuite(ctx context.Context, suiteID string, opts runOptions) error {
	suite, err := testSuiteReader.GetTestSuite(ctx, suiteID)
	if err != nil {
		return err
	}

	list, err := testCaseService.ListTestCases(ctx, aiauthoring.ListOptions{TestSuiteID: suiteID})
	if err != nil {
		return fmt.Errorf("failed to list the test cases of suite %q: %w", suite.Name, err)
	}
	if len(list.Items) == 0 {
		return fmt.Errorf("test suite %q has no test cases", suite.Name)
	}

	if opts.OutFormat == TextOutput {
		fmt.Printf("Running test suite %q with %d test case(s)\n", suite.Name, len(list.Items))
	}

	// A suite is run client-side: the backend has no suite-level run
	// endpoint, so every test case is started individually.
	type startedRun struct {
		tc  aiauthoring.TestCase
		run aiauthoring.TestCaseRun
		err error
	}
	started := make([]startedRun, 0, len(list.Items))
	for _, tc := range list.Items {
		run, err := startRun(ctx, tc, opts)
		started = append(started, startedRun{tc: tc, run: run, err: err})
		if err != nil {
			log.Error().Err(err).Str("testCase", tc.Name).Msg("Failed to start the test case run.")
			continue
		}
		if opts.OutFormat == TextOutput {
			fmt.Printf("Started run %s of test case %q\n", run.ID, tc.Name)
		}
	}

	if opts.Async {
		if opts.OutFormat == JSONOutput {
			runs := make([]aiauthoring.TestCaseRun, 0, len(started))
			for _, s := range started {
				if s.err == nil {
					runs = append(runs, s.run)
				}
			}
			return renderJSON(runs)
		}
		return nil
	}

	// Poll all runs concurrently; they execute concurrently in the cloud.
	results := make([][]runResult, len(started))
	var wg sync.WaitGroup
	for i, s := range started {
		if s.err != nil {
			results[i] = []runResult{{
				Name:   s.tc.Name,
				Status: job.StateError,
				Error:  s.err.Error(),
			}}
			continue
		}

		wg.Add(1)
		go func(i int, s startedRun) {
			defer wg.Done()
			final, err := pollRun(ctx, s.tc.ID, s.run, opts.Timeout)
			if err != nil {
				results[i] = []runResult{{
					Name:   s.tc.Name,
					Status: job.StateError,
					Error:  err.Error(),
				}}
				return
			}
			results[i] = toRunResults(final)
		}(i, s)
	}
	wg.Wait()

	var merged []runResult
	for _, r := range results {
		merged = append(merged, r...)
	}

	return reportResults(merged, opts.OutFormat)
}

// startRun starts a run of the given test case, targeting the flag-provided
// platforms if any, otherwise the test case's authored target.
func startRun(ctx context.Context, tc aiauthoring.TestCase, opts runOptions) (aiauthoring.TestCaseRun, error) {
	targets := opts.Targets
	clearTunnel := false
	if tc.RunSettings != nil {
		if targets == nil && tc.RunSettings.PrimaryTarget != nil {
			targets = []aiauthoring.RunTarget{*tc.RunSettings.PrimaryTarget}
		}
		// A test case authored with an empty (rather than absent) tunnel
		// name fails to start with SC_TUNNEL_NOT_FOUND unless the tunnel is
		// explicitly cleared.
		if opts.Tunnel == "" && tc.RunSettings.SCTunnelName != nil && *tc.RunSettings.SCTunnelName == "" {
			log.Debug().Str("testCase", tc.Name).Msg("Clearing the empty tunnel name stored in the test case's run settings.")
			clearTunnel = true
		}
	}

	run, err := testCaseRunner.RunTestCase(ctx, tc.ID, aiauthoring.RunRequest{
		BuildName:    opts.Build,
		SCTunnelName: opts.Tunnel,
		ClearTunnel:  clearTunnel,
		Targets:      targets,
	})
	if err != nil {
		return aiauthoring.TestCaseRun{}, fmt.Errorf("failed to start the test case run: %w", err)
	}
	return run, nil
}

// pollRun polls the run until every job reaches a final state or the timeout
// expires. The job ids returned by the run request are internal to the AI
// Authoring backend and are not resolvable via the regular job APIs; the run
// resource itself is the authoritative source of job progress, including the
// Sauce job id and URL, which it reports once the job has been scheduled.
func pollRun(ctx context.Context, testCaseID string, run aiauthoring.TestCaseRun, timeout time.Duration) (aiauthoring.TestCaseRun, error) {
	log.Info().Str("runID", run.ID).Msg("Waiting for the run to finish. Device allocation can take a few minutes.")

	ticker := time.NewTicker(runPollInterval)
	defer ticker.Stop()
	deathclock := time.NewTimer(timeout)
	defer deathclock.Stop()

	current := run
	for {
		if runDone(current) {
			return current, nil
		}

		select {
		case <-ctx.Done():
			return current, ctx.Err()
		case <-deathclock.C:
			return current, fmt.Errorf("run did not finish within %s; check its status at %s", timeout, current.TestURL)
		case <-ticker.C:
			r, err := testCaseRunner.GetTestCaseRun(ctx, testCaseID, run.ID)
			if err != nil {
				// Transient lookup errors should not kill a run that is
				// still executing in the cloud.
				log.Warn().Err(err).Msg("Unable to fetch the run status. Retrying.")
				continue
			}
			current = r
		}
	}
}

func runDone(run aiauthoring.TestCaseRun) bool {
	for _, j := range run.Jobs {
		if !j.Done() {
			return false
		}
	}
	return len(run.Jobs) > 0
}

func toRunResults(run aiauthoring.TestCaseRun) []runResult {
	var results []runResult
	for _, j := range run.Jobs {
		status := job.StateFailed
		if j.Passed() {
			status = job.StatePassed
		} else if j.Error != "" {
			status = job.StateError
		}

		// The backend omits the job URL for VDC jobs; construct it from the
		// Sauce job id instead.
		u := j.URL
		if u == "" && j.SauceJobID != "" {
			u = fmt.Sprintf("%s/tests/%s", appURL, j.SauceJobID)
		}

		results = append(results, runResult{
			Name:       j.Name,
			SauceJobID: j.SauceJobID,
			Status:     status,
			Passed:     j.Passed(),
			URL:        u,
			Error:      j.Error,
		})
	}
	return results
}

func reportResults(results []runResult, outFormat string) error {
	if outFormat == JSONOutput {
		if err := renderJSON(results); err != nil {
			return err
		}
	} else {
		renderRunResults(results)
	}

	for _, r := range results {
		if !r.Passed {
			return errors.New("some test case jobs have failed")
		}
	}

	return nil
}

func renderRunResults(results []runResult) {
	t := table.NewWriter()
	t.SetStyle(tables.DefaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"Name", "Status", "URL",
	})

	errCount := 0
	for _, r := range results {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			r.Name,
			r.Status,
			r.URL,
		})
		if !r.Passed {
			errCount++
		}
		if r.Error != "" {
			log.Error().Str("name", r.Name).Msg(r.Error)
		}
	}

	t.AppendFooter(table.Row{
		fmt.Sprintf("%d of %d job(s) passed", len(results)-errCount, len(results)),
	})

	fmt.Println(t.Render())
}
