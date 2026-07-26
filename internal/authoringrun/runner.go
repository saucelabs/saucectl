package authoringrun

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
)

// pollInterval is how often we check on a job's status while waiting for it
// to finish.
var pollInterval = 5 * time.Second

// Runner executes AI-authored test suites: for each suite in the project, it
// asks Sauce Labs which test cases currently belong to it, then runs them
// with client-side bounded concurrency (mirroring the worker-pool pattern in
// internal/saucecloud/cloud.go), polling each job to completion and feeding
// results into the same report.Reporter pipeline every other framework uses.
//
// See docs/authoring-run-integration-rfc.md for the design this implements.
type Runner struct {
	Project    Project
	Client     authoring.Service
	JobService job.Service
	Region     region.Region
	Reporters  []report.Reporter

	Async    bool
	FailFast bool
}

// testCaseUnit is one unit of work for the worker pool: a single test case
// to run and wait on.
type testCaseUnit struct {
	suiteName string
	timeout   time.Duration
	buildName string
	testCase  authoring.TestCase
}

// unitResult is what a worker reports back after running (and, unless
// Async, polling) a single test case.
type unitResult struct {
	suiteName string
	testCase  authoring.TestCase
	jobs      []polledJob
	skipped   bool
	err       error
	startTime time.Time
	endTime   time.Time
}

type polledJob struct {
	authoringJob authoring.TestCaseJob
	job          job.Job
}

// RunProject runs every suite defined in the project.
func (r *Runner) RunProject(ctx context.Context) (int, error) {
	if r.Project.DryRun {
		r.printDryRun()
		return 0, nil
	}

	passed := true
	for _, s := range r.Project.Suites {
		suitePassed, err := r.runSuite(ctx, s)
		if err != nil {
			log.Error().Err(err).Str("suite", s.Name).Msg("Failed to run suite.")
			passed = false
			continue
		}
		if !suitePassed {
			passed = false
		}
	}

	if passed {
		return 0, nil
	}
	return 1, nil
}

func (r *Runner) printDryRun() {
	fmt.Println("\nThe following authoring suites would have run:")
	for _, s := range r.Project.Suites {
		fmt.Printf("  - %s (test suite %s)\n", s.Name, s.TestSuiteID)
	}
	fmt.Println()
}

// runSuite fetches every test case currently in s.TestSuiteID and runs them
// with at most r.Project.Sauce.Concurrency in flight at a time. Returns
// whether every test case in the suite passed.
func (r *Runner) runSuite(ctx context.Context, s Suite) (bool, error) {
	testCases, err := r.listAllTestCases(ctx, s.TestSuiteID, s.Tags)
	if err != nil {
		return false, fmt.Errorf("failed to list test cases for suite %s (%s): %w", s.Name, s.TestSuiteID, err)
	}
	if len(testCases) == 0 {
		log.Warn().Str("suite", s.Name).Msg("Suite has no test cases; nothing to run.")
		return true, nil
	}

	buildName := s.buildName(r.Project.Sauce.Metadata.Build)

	ccy := r.Project.Sauce.Concurrency
	if ccy < 1 {
		ccy = 1
	}
	if ccy > len(testCases) {
		ccy = len(testCases)
	}

	units := make(chan testCaseUnit, len(testCases))
	results := make(chan unitResult, len(testCases))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < ccy; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.worker(runCtx, units, results)
		}()
	}

	for _, tc := range testCases {
		units <- testCaseUnit{
			suiteName: s.Name,
			timeout:   s.Timeout,
			buildName: buildName,
			testCase:  tc,
		}
	}
	close(units)

	passed := r.collectResults(len(testCases), results, cancel)
	wg.Wait()

	return passed, nil
}

// worker consumes test case units one at a time until the channel is closed,
// running (and, unless Async, polling) each one before moving on to the
// next -- this is what gives us "N in flight, next one starts as soon as a
// slot frees up" rather than fixed static batches.
func (r *Runner) worker(ctx context.Context, units <-chan testCaseUnit, results chan<- unitResult) {
	for u := range units {
		if ctx.Err() != nil {
			results <- unitResult{suiteName: u.suiteName, testCase: u.testCase, skipped: true}
			continue
		}

		results <- r.runUnit(ctx, u)
	}
}

func (r *Runner) runUnit(ctx context.Context, u testCaseUnit) unitResult {
	start := time.Now()

	run, err := r.Client.RunTestCase(ctx, u.testCase.ID, authoring.RunTestCaseOptions{
		BuildName: u.buildName,
	})
	if err != nil {
		return unitResult{
			suiteName: u.suiteName,
			testCase:  u.testCase,
			err:       fmt.Errorf("failed to run test case %s (%s): %w", u.testCase.Name, u.testCase.ID, err),
			startTime: start,
			endTime:   time.Now(),
		}
	}

	if r.Async {
		return unitResult{
			suiteName: u.suiteName,
			testCase:  u.testCase,
			startTime: start,
			endTime:   time.Now(),
		}
	}

	timeout := u.timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	polled := make([]polledJob, 0, len(run.Jobs))
	for _, j := range run.Jobs {
		jobID := j.SauceJobID
		if jobID == "" {
			jobID = j.ID
		}

		finalJob, err := r.JobService.PollJob(ctx, jobID, pollInterval, timeout, j.IsRDC)
		if err != nil {
			log.Error().Err(err).Str("testCase", u.testCase.Name).Str("job", jobID).Msg("Failed to poll job.")
		}
		polled = append(polled, polledJob{authoringJob: j, job: finalJob})
	}

	return unitResult{
		suiteName: u.suiteName,
		testCase:  u.testCase,
		jobs:      polled,
		startTime: start,
		endTime:   time.Now(),
	}
}

// collectResults drains exactly `expected` results, feeds them into the
// configured reporters, and returns whether everything passed. If FailFast
// is set, the first failure cancels cancel(), which causes any units the
// worker pool hasn't started yet to be reported as skipped instead of run.
func (r *Runner) collectResults(expected int, results <-chan unitResult, cancel context.CancelFunc) bool {
	passed := true

	for i := 0; i < expected; i++ {
		res := <-results

		if res.skipped {
			continue
		}

		displayName := fmt.Sprintf("%s - %s", res.suiteName, res.testCase.Name)

		if res.err != nil {
			passed = false
			log.Error().Err(res.err).Str("testCase", res.testCase.Name).Str("suite", res.suiteName).Msg("Test case run failed.")
			for _, rep := range r.Reporters {
				rep.Add(report.TestResult{
					Name:      displayName,
					Status:    job.StateError,
					StartTime: res.startTime,
					EndTime:   res.endTime,
					Duration:  res.endTime.Sub(res.startTime),
				})
			}
			if r.FailFast {
				cancel()
			}
			continue
		}

		if len(res.jobs) == 0 {
			// Async: nothing to report on yet beyond "it was queued".
			for _, rep := range r.Reporters {
				rep.Add(report.TestResult{
					Name:      displayName,
					Status:    job.StateQueued,
					StartTime: res.startTime,
					EndTime:   res.endTime,
					Duration:  res.endTime.Sub(res.startTime),
				})
			}
			continue
		}

		for _, pj := range res.jobs {
			status := pj.job.TotalStatus()
			if status == "" {
				status = job.StateUnknown
			}
			if !pj.job.IsSuccessful() {
				passed = false
			}

			name := displayName
			if pj.authoringJob.Name != "" {
				name = fmt.Sprintf("%s - %s", res.suiteName, pj.authoringJob.Name)
			}

			platform := pj.job.OS
			if pj.job.OSVersion != "" {
				platform = fmt.Sprintf("%s %s", platform, pj.job.OSVersion)
			}

			tr := report.TestResult{
				Name:       name,
				Duration:   res.endTime.Sub(res.startTime),
				StartTime:  res.startTime,
				EndTime:    res.endTime,
				Status:     status,
				Browser:    pj.job.BrowserName,
				Platform:   platform,
				DeviceName: pj.job.DeviceName,
				URL:        pj.job.URL,
				RDC:        pj.authoringJob.IsRDC,
				TimedOut:   pj.job.TimedOut,
				Attempts: []report.Attempt{{
					ID:        pj.job.ID,
					Duration:  res.endTime.Sub(res.startTime),
					StartTime: res.startTime,
					EndTime:   res.endTime,
					Status:    status,
				}},
			}
			if tr.URL == "" {
				tr.URL = pj.authoringJob.URL
			}

			for _, rep := range r.Reporters {
				rep.Add(tr)
			}

			if pj.job.TimedOut {
				passed = false
				if r.FailFast {
					cancel()
				}
			}
		}
	}

	for _, rep := range r.Reporters {
		rep.Render()
	}

	return passed
}

// listAllTestCases pages through every test case in the suite, optionally
// narrowed to those matching tags (this is what Suite.Tags is for: filtering
// which of the suite's test cases run, not tagging the run call itself --
// the run endpoint has no such parameter).
func (r *Runner) listAllTestCases(ctx context.Context, testSuiteID string, tags []string) ([]authoring.TestCase, error) {
	const pageSize = 100

	var all []authoring.TestCase
	skip := 0

	for {
		page, total, err := r.Client.ListTestCases(ctx, authoring.ListTestCaseOptions{
			TestSuiteID: testSuiteID,
			Tags:        tags,
			Skip:        skip,
			Limit:       pageSize,
		})
		if err != nil {
			return nil, err
		}

		all = append(all, page...)

		if len(page) == 0 || len(all) >= total {
			break
		}
		skip += len(page)
	}

	return all, nil
}

// buildName returns the build name to tag every job with for this suite run.
// Falls back to a suite-scoped, timestamped default so jobs are still
// grouped together in the Sauce Labs UI even if the user hasn't set
// sauce::metadata::build.
func (s Suite) buildName(projectBuild string) string {
	if projectBuild != "" {
		return projectBuild
	}
	return fmt.Sprintf("authoring-%s-%d", s.Name, time.Now().Unix())
}
