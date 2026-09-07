package authoring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

// DefaultPollInterval is how often a run is polled for completion. Observed
// runs of a short journey take ~20 s and report success on the run resource
// ~4 s before the Sauce job completes (research Open-1), so five seconds
// keeps requests modest without adding noticeable latency.
const DefaultPollInterval = 5 * time.Second

// stopJobTimeout bounds the attempt to stop jobs after we have stopped
// waiting. It runs on a context detached from the interrupted one, so it needs
// a deadline of its own to honour the promise that nothing waits for ever.
const stopJobTimeout = 30 * time.Second

// JobStopper stops a Sauce job. It is the part of saucecloud.JobService the
// runner needs to avoid leaving work running after we stop watching it.
type JobStopper interface {
	StopJob(ctx context.Context, jobID string, realDevice bool) (job.Job, error)
}

// ArtifactDownloader is the part of saucecloud.JobService the runner needs.
// It takes a job.Job so the shared skip rules (when: fail/pass, timed out,
// unfinished) apply exactly as they do for every other kind.
type ArtifactDownloader interface {
	DownloadArtifacts(ctx context.Context, j job.Job, isLastAttempt bool) []string
}

// Runner executes a `kind: authoring` project: it resolves suites to test
// cases, starts one run per case under a bounded worker pool, polls each run
// to completion and hands one result per job to the shared reporters.
//
// It is deliberately not a saucecloud.CloudRunner: that type is built around
// job.StartOptions (WebDriver job creation), whereas these runs are triggered
// through the authoring service. The runner also deliberately does not copy
// internal/apitest's shape, which starts every test at once and shares a
// package-level poll variable across suites (research R-012).
type Runner struct {
	Project    Project
	TestCases  TestCaseService
	TestSuites TestSuiteService
	// Artifacts downloads job assets after a run; nil disables downloads.
	Artifacts ArtifactDownloader
	// Stopper stops jobs when we stop waiting for them, so an interrupted or
	// timed-out run does not keep consuming the organisation's concurrency.
	// nil disables stopping.
	Stopper JobStopper
	// Builds resolves the build link shown under the results table; nil
	// leaves it out.
	Builds build.Service
	// Tunnels validates tunnel readiness before anything starts.
	Tunnels   tunnel.Service
	Region    region.Region
	Reporters []report.Reporter
	// Async starts every run and returns without waiting.
	Async bool
	// PollInterval overrides DefaultPollInterval; tests set it very low.
	PollInterval time.Duration
}

// ResolvedCase pairs a configured suite with one test case it expands to.
type ResolvedCase struct {
	Suite    Suite
	TestCase TestCase
}

// runResult is what one worker produces for one resolved case.
type runResult struct {
	Case ResolvedCase
	// Run is the last state seen. Its Jobs are empty when the start failed.
	Run Run
	// Err is a start failure or a fatal poll failure.
	Err error
	// TimedOut is set when the suite timeout expired before the run ended.
	TimedOut bool
	// Interrupted is set when the context ended before the run did; the run
	// continues on Sauce Labs.
	Interrupted bool
	StartTime   time.Time
	EndTime     time.Time
}

// RunProject runs the project and returns the process exit code: 0 only when
// every run passed (or was started, under Async), 1 otherwise.
func (r *Runner) RunProject(ctx context.Context) (int, error) {
	// The owner is deliberately not passed: runCase sends only scTunnelName,
	// so validating with an owner would check a colleague's tunnel and then
	// report ready for a run that fails with SC_TUNNEL_NOT_FOUND. Validate
	// says exactly what the run will do. Validate() warns that the owner is
	// ignored.
	if err := tunnel.Validate(
		ctx,
		r.Tunnels,
		r.Project.Sauce.Tunnel.Name,
		"",
		tunnel.NoneFilter,
		r.Project.DryRun,
		r.Project.Sauce.Tunnel.Timeout,
	); err != nil {
		return 1, err
	}

	cases, err := r.ResolveTestCases(ctx)
	if err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		printDryRun(cases)
		return 0, nil
	}

	if r.runCases(ctx, cases) {
		return 0, nil
	}
	return 1, nil
}

// ResolveTestCases expands every configured suite into concrete test cases.
// Resolution is read-only, which is what lets --dry-run show exactly what
// would execute.
func (r *Runner) ResolveTestCases(ctx context.Context) ([]ResolvedCase, error) {
	var out []ResolvedCase
	for _, s := range r.Project.Suites {
		cases, err := r.resolveSuite(ctx, s)
		if err != nil {
			return nil, fmt.Errorf("suite %q: %w", s.Name, err)
		}
		if len(cases) == 0 {
			log.Warn().Str("suite", s.Name).Msg("Suite resolved to no test cases.")
		}
		for _, tc := range cases {
			out = append(out, ResolvedCase{Suite: s, TestCase: tc})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no test cases to run")
	}
	return out, nil
}

// resolveSuite resolves one suite entry: explicit cases are fetched one by
// one; a suite reference is expanded through the testSuiteId filter, which
// matches the suite's own count exactly (research R-003).
func (r *Runner) resolveSuite(ctx context.Context, s Suite) ([]TestCase, error) {
	if len(s.TestCases) > 0 {
		cases := make([]TestCase, 0, len(s.TestCases))
		for _, id := range s.TestCases {
			tc, err := r.TestCases.GetTestCase(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("test case %s: %w", id, err)
			}
			cases = append(cases, tc)
		}
		return cases, nil
	}

	suiteID := s.TestSuiteID
	if s.TestSuiteName != "" {
		id, err := r.findSuiteByName(ctx, s.TestSuiteName)
		if err != nil {
			return nil, err
		}
		suiteID = id
	}

	return ListAll(ctx, DefaultPageSize, func(ctx context.Context, lo ListOptions) (List[TestCase], error) {
		return r.TestCases.ListTestCases(ctx, ListTestCasesOptions{
			ListOptions:  lo,
			TestSuiteIDs: []string{suiteID},
			Tags:         s.Tags,
		})
	})
}

// findSuiteByName resolves a suite by exact name. The service's search is a
// case-insensitive substring match inside words ("demo" matches "Saucedemo -
// Checkout flow"), so the exact comparison happens here, and an ambiguous
// name is an error rather than "first result wins".
func (r *Runner) findSuiteByName(ctx context.Context, name string) (string, error) {
	all, err := ListAll(ctx, DefaultPageSize, func(ctx context.Context, lo ListOptions) (List[TestSuite], error) {
		return r.TestSuites.ListTestSuites(ctx, ListTestSuitesOptions{ListOptions: lo, Search: name})
	})
	if err != nil {
		return "", fmt.Errorf("looking up test suite %q: %w", name, err)
	}

	var ids []string
	for _, s := range all {
		if s.Name == name {
			ids = append(ids, s.ID)
		}
	}
	switch len(ids) {
	case 0:
		return "", fmt.Errorf("no test suite is named %q (the match is exact and case-sensitive)", name)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("%d test suites are named %q; reference one by testSuiteId instead: %s", len(ids), name, strings.Join(ids, ", "))
	}
}

// printDryRun lists what would run without starting anything.
func printDryRun(cases []ResolvedCase) {
	fmt.Println("\nThe following test cases would have run:")
	for _, c := range cases {
		targets := "stored run targets"
		if n := len(c.Suite.Targets); n > 0 {
			targets = fmt.Sprintf("%d configured target(s)", n)
		}
		fmt.Printf("  - %s: %s (%s) on %s\n", c.Suite.Name, c.TestCase.Name, c.TestCase.ID, targets)
	}
	fmt.Println()
}

// runCases starts one run per case under a semaphore sized from
// sauce.concurrency, then collects results. Unlike apitest's unbounded
// fan-out, no more than the configured number of runs are ever in flight
// (SC-011): an authored run consumes real VM or device capacity.
func (r *Runner) runCases(ctx context.Context, cases []ResolvedCase) bool {
	ccy := r.Project.Sauce.Concurrency
	if ccy < 1 {
		ccy = 1
	}
	totalJobs := 0
	for _, c := range cases {
		totalJobs += expectedJobs(c)
	}
	log.Info().Int("concurrency", ccy).Int("testCases", len(cases)).Int("jobs", totalJobs).
		Msg("Starting AI-authored test runs.")

	results := make(chan runResult, len(cases))
	sem := make(chan struct{}, ccy)
	// gate serialises acquisition. Without it two cases each needing two of
	// two slots would take one apiece and wait for ever for the other's —
	// a deadlock, not merely unfair. Holding the gate while waiting means
	// one case queues behind another, which is the price of a correct
	// ceiling and is invisible at these sizes.
	gate := make(chan struct{}, 1)
	for _, c := range cases {
		go func(c ResolvedCase) {
			// The limit counts Sauce jobs, not runs: one run fans out to one
			// job per target, so a per-run semaphore would let a suite with
			// four targets put four times the configured load on the
			// organisation's capacity (SC-011). Weight each case by the jobs
			// it will start, capped at the whole budget so a case needing
			// more than that still runs — alone — rather than never.
			weight := expectedJobs(c)
			if weight > ccy {
				weight = ccy
			}
			if !acquire(ctx, gate, sem, weight) {
				results <- runResult{Case: c, Err: ctx.Err(), Interrupted: true, StartTime: time.Now(), EndTime: time.Now()}
				return
			}
			defer release(sem, weight)
			results <- r.runCase(ctx, c)
		}(c)
	}

	return r.collectResults(ctx, results, len(cases))
}

// expectedJobs reports how many Sauce jobs one resolved case will start. The
// service starts one per target: the suite's targets when it overrides them,
// otherwise the case's own stored run targets, otherwise its single primary
// target. Everything needed is already in hand, so this costs no request.
func expectedJobs(c ResolvedCase) int {
	if n := len(c.Suite.Targets); n > 0 {
		return n
	}
	if n := len(c.TestCase.RunSettings.RunTargets); n > 0 {
		return n
	}
	return 1
}

// acquire takes n slots, or reports false if the context ended first.
//
// The gate ensures only one caller collects slots at a time, so a caller
// asking for n <= capacity always eventually gets them as holders finish.
// Collecting slots concurrently would let two callers each hold part of the
// budget and deadlock waiting for the rest.
func acquire(ctx context.Context, gate, sem chan struct{}, n int) bool {
	select {
	case gate <- struct{}{}:
	case <-ctx.Done():
		return false
	}
	defer func() { <-gate }()

	for i := 0; i < n; i++ {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			release(sem, i)
			return false
		}
	}
	return true
}

// release returns n slots.
func release(sem chan struct{}, n int) {
	for i := 0; i < n; i++ {
		<-sem
	}
}

// runCase starts one run and, unless Async, polls it to completion.
func (r *Runner) runCase(ctx context.Context, c ResolvedCase) runResult {
	res := runResult{Case: c, StartTime: time.Now()}

	// No configured tunnel means an explicit null on the wire, which also
	// clears a stored (possibly empty and therefore broken) tunnel name on
	// the case (research Open-3).
	opts := RunOptions{
		BuildName:  r.Project.Sauce.Metadata.Build,
		TunnelName: r.Project.Sauce.Tunnel.Name,
		Targets:    c.Suite.Targets,
	}

	run, err := r.TestCases.RunTestCase(ctx, c.TestCase.ID, "", opts)
	if err != nil {
		res.Err = fmt.Errorf("failed to start run: %w", err)
		res.EndTime = time.Now()
		return res
	}
	// The run resource is polled with its own testCaseId (research R-004).
	// If the start response omitted it, fall back to the identifier we asked
	// with: polling an empty one 404s, and isFatalPollError treats 404 as
	// transient, so it would burn the whole suite timeout in silence.
	if run.TestCaseID == "" {
		run.TestCaseID = c.TestCase.ID
	}
	res.Run = run
	for _, j := range run.Jobs {
		log.Info().
			Str("suite", c.Suite.Name).
			Str("testCase", c.TestCase.Name).
			Str("url", r.jobURL(j.SauceJobID)).
			Msg("Run started.")
	}

	if r.Async {
		res.EndTime = time.Now()
		return res
	}

	latest, timedOut, err := r.pollRun(ctx, run, c.Suite.Timeout)
	res.Run, res.TimedOut, res.EndTime = latest, timedOut, time.Now()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			res.Interrupted = true
		}
		res.Err = err
	}

	// We have stopped watching, so stop the work: otherwise a cancelled or
	// timed-out run keeps a VM or device busy for its full duration, which is
	// what every other kind avoids (internal/saucecloud/cloud.go).
	if res.TimedOut || res.Interrupted {
		r.stopRun(res.Run, c)
	}
	return res
}

// stopRun asks the service to stop every job of a run we are no longer
// waiting for. Errors are ignored, as they are for the other kinds: a job may
// already have ended, or be in a state that cannot be stopped, and either way
// there is nothing to do about it.
func (r *Runner) stopRun(run Run, c ResolvedCase) {
	if r.Stopper == nil {
		return
	}
	// The caller's context is already cancelled on Ctrl-C, so stopping needs
	// one that outlives it. This mirrors the localCtx in saucecloud, whose
	// comment says it exists so jobs are not left abandoned.
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), stopJobTimeout)
	defer cancel()

	for _, j := range run.Jobs {
		if j.SauceJobID == "" {
			continue
		}
		log.Info().
			Str("suite", c.Suite.Name).
			Str("testCase", c.TestCase.Name).
			Str("job", j.SauceJobID).
			Msg("Attempting to stop job...")
		_, _ = r.Stopper.StopJob(stopCtx, j.SauceJobID, j.RealDevice())
	}
}

// pollRun polls the run until every job has an outcome, the suite timeout
// expires, or the context ends. It polls before the first wait and returns
// at once when the start response is already terminal, so it behaves
// correctly whether the service answers asynchronously (observed) or
// synchronously. Transient failures (network, 5xx, and 404 which may be
// propagation lag) are retried until the deadline; other 4xx are fatal. The
// timeout is an argument, never package state, so one suite's setting
// cannot leak into another's (research R-012).
func (r *Runner) pollRun(ctx context.Context, run Run, timeout time.Duration) (Run, bool, error) {
	if run.Done() {
		return run, false, nil
	}

	interval := r.PollInterval
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	if timeout <= 0 {
		timeout = DefaultSuiteTimeout
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		latest, err := r.TestCases.GetRun(ctx, run.TestCaseID, run.ID)
		switch {
		case err == nil:
			run = latest
			if run.Done() {
				return run, false, nil
			}
		case ctx.Err() != nil:
			return run, false, ctx.Err()
		case IsFatalPollError(err):
			return run, false, fmt.Errorf("failed to poll run %s: %w", run.ID, err)
		default:
			log.Debug().Err(err).Str("run", run.ID).Msg("Transient error while polling run; retrying.")
		}

		select {
		case <-ctx.Done():
			return run, false, ctx.Err()
		case <-deadline.C:
			return run, true, nil
		case <-time.After(interval):
		}
	}
}

// IsFatalPollError reports whether a poll error cannot be recovered by
// waiting: a 4xx other than 404. 404 is tolerated because a freshly started
// run — or a freshly accepted generation task — might not be readable for a
// moment; the caller's deadline still bounds the retrying. Exported so the
// generate wait applies the same classification as the run poll; two loops in
// one feature disagreeing about which errors are fatal is how a user ends up
// told to keep polling a task that does not exist.
func IsFatalPollError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 && apiErr.HTTPStatus != 404
}

// collectResults drains the results, feeds the reporters and renders them.
// It returns whether everything passed.
func (r *Runner) collectResults(ctx context.Context, results <-chan runResult, expected int) bool {
	passed := true
	var inProgress atomic.Int32
	inProgress.Store(int32(expected))

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				log.Info().Msgf("Runs in progress: %d", inProgress.Load())
			}
		}
	}()

	wantJUnit := report.IsArtifactRequired(r.Reporters, report.JUnitArtifact)
	buildURLs := map[build.Source]string{}

	for i := 0; i < expected; i++ {
		res := <-results
		inProgress.Add(-1)

		r.logResult(res)
		if !r.resultPassed(res) {
			passed = false
		}
		for _, tr := range r.toTestResults(ctx, res, wantJUnit, buildURLs) {
			for _, rep := range r.Reporters {
				rep.Add(tr)
			}
		}
	}
	close(done)

	for _, rep := range r.Reporters {
		rep.Render()
	}
	return passed
}

// resultPassed decides the exit code contribution of one result. Under Async
// a started run counts as passing because its outcome is unknown by
// definition (FR-009).
func (r *Runner) resultPassed(res runResult) bool {
	if res.Err != nil || res.TimedOut || res.Interrupted {
		return false
	}
	if r.Async {
		return true
	}
	return res.Run.Passed()
}

// logResult writes one line per finished case, telling the user how to
// follow a run that is still going when the wait ended early.
func (r *Runner) logResult(res runResult) {
	ev := log.Info()
	msg := "Run finished."
	checkHint := fmt.Sprintf("saucectl authoring testcases get-run %s %s", res.Run.TestCaseID, res.Run.ID)

	switch {
	case res.Interrupted && res.Run.ID == "":
		ev = log.Warn()
		msg = "Run was not started: interrupted."
	case res.Interrupted:
		ev = log.Warn()
		msg = "Interrupted; stopping the run on Sauce Labs. Check it with: " + checkHint
	case res.Err != nil:
		ev = log.Error().Err(res.Err)
		msg = "Run failed."
	case res.TimedOut:
		ev = log.Error()
		msg = "Timed out waiting; stopping the run on Sauce Labs. Check it with: " + checkHint
	case r.Async:
		msg = "Run started (async). Check it with: " + checkHint
	case !res.Run.Passed():
		ev = log.Error()
		msg = "Run finished with failures."
	}

	ev.Str("suite", res.Case.Suite.Name).Str("testCase", res.Case.TestCase.Name).Str("run", res.Run.ID).Msg(msg)
}

// toTestResults turns one run into one report.TestResult per job — one per
// browser or device, not one per suite (FR-003). A run that never produced
// jobs (start failure, interruption before start) yields a single failed or
// in-progress result so it is never silently absent from the report.
func (r *Runner) toTestResults(ctx context.Context, res runResult, wantJUnit bool, buildURLs map[build.Source]string) []report.TestResult {
	name := res.Case.Suite.Name + " - " + res.Case.TestCase.Name
	duration := res.EndTime.Sub(res.StartTime)

	if len(res.Run.Jobs) == 0 {
		// Only a start failure is a failure. An interruption, or an accepted
		// asynchronous run whose jobs the service has not reported yet, is
		// still in progress — reporting it as failed would contradict the
		// zero exit code resultPassed returns under Async.
		status := job.StateFailed
		if res.Interrupted || (r.Async && res.Err == nil) {
			status = job.StateInProgress
		}
		tr := report.TestResult{
			Name:      name,
			Duration:  duration,
			StartTime: res.StartTime,
			EndTime:   res.EndTime,
			Status:    status,
			TimedOut:  res.TimedOut,
			Attempts:  []report.Attempt{{Duration: duration, StartTime: res.StartTime, EndTime: res.EndTime, Status: status}},
		}
		if wantJUnit {
			tr.Attempts[0].TestSuites = synthesizeJUnit(res.Case, RunJob{Error: errString(res.Err)}, status, duration)
		}
		return []report.TestResult{tr}
	}

	out := make([]report.TestResult, 0, len(res.Run.Jobs))
	for _, j := range res.Run.Jobs {
		status := jobState(j)
		browser, platform, device := DescribeCapabilities(j.Target.Capabilities)

		tr := report.TestResult{
			Name:       name,
			Duration:   duration,
			StartTime:  res.StartTime,
			EndTime:    res.EndTime,
			Status:     status,
			Browser:    browser,
			Platform:   platform,
			DeviceName: device,
			URL:        r.jobURL(j.SauceJobID),
			RunID:      res.Run.ID,
			// RDC routes the row into the real-device table, which has its
			// own build link (internal/report/buildtable).
			RDC: j.RealDevice(),
			// TimedOut is what makes the table count an unfinished run as an
			// error rather than merely in progress (internal/report/table).
			TimedOut: res.TimedOut,
			Attempts: []report.Attempt{{
				ID:        j.SauceJobID,
				Duration:  duration,
				StartTime: res.StartTime,
				EndTime:   res.EndTime,
				Status:    status,
			}},
		}

		if wantJUnit {
			tr.Attempts[0].TestSuites = synthesizeJUnit(res.Case, j, status, duration)
		}
		if !r.Async && j.SauceJobID != "" {
			tr.BuildURL = r.buildURL(ctx, j, buildURLs)
		}
		if !r.Async && r.Artifacts != nil && j.Done() && j.SauceJobID != "" {
			tr.Artifacts = r.downloadArtifacts(ctx, name, j, status, res.TimedOut)
		}

		out = append(out, tr)
	}
	return out
}

// jobState maps a job's inferred outcome onto the shared result states.
// There is no status on the wire; nil success is "not yet reported"
// (research R-005).
func jobState(j RunJob) string {
	switch {
	case j.Passed():
		return job.StatePassed
	case j.Done():
		return job.StateFailed
	default:
		return job.StateInProgress
	}
}

// jobURL derives the dashboard link for one job in this runner's region.
func (r *Runner) jobURL(sauceJobID string) string {
	return JobURL(r.Region, sauceJobID)
}

// buildURL resolves the build link through the build service by job ID,
// caching per source: every job of this invocation shares the build. The
// service decorates the build name ("X" becomes "X - 1"), which is why the
// lookup is by job and never by name.
func (r *Runner) buildURL(ctx context.Context, j RunJob, cache map[build.Source]string) string {
	if r.Builds == nil {
		return ""
	}
	source := build.SourceVDC
	if j.RealDevice() {
		source = build.SourceRDC
	}
	if u, ok := cache[source]; ok {
		return u
	}

	b, err := r.Builds.GetBuild(ctx, build.GetBuildOptions{ID: j.SauceJobID, Source: source, ByJob: true})
	if err != nil {
		log.Debug().Err(err).Str("job", j.SauceJobID).Msg("Unable to resolve the build link.")
		return ""
	}
	cache[source] = b.URL
	return b.URL
}

// downloadArtifacts hands the job to the shared downloader with the fields
// its skip rules read: a done state, the pass flag and the timeout flag.
func (r *Runner) downloadArtifacts(ctx context.Context, name string, j RunJob, status string, timedOut bool) []report.Artifact {
	files := r.Artifacts.DownloadArtifacts(ctx, job.Job{
		ID:       j.SauceJobID,
		Name:     name,
		Passed:   j.Passed(),
		Status:   status,
		IsRDC:    j.RealDevice(),
		TimedOut: timedOut,
	}, true)

	arts := make([]report.Artifact, 0, len(files))
	for _, f := range files {
		arts = append(arts, report.Artifact{FilePath: f})
	}
	return arts
}

// synthesizeJUnit builds the JUnit content for one job. Authored runs publish
// no junit.xml asset (research Open-2), and the JUnit reporter renders
// <testcase> elements only from Attempt.TestSuites, so without this the
// report would be a set of empty <testsuite> containers that downstream
// consumers show as "no tests" (research R-011, FR-006, SC-008).
func synthesizeJUnit(c ResolvedCase, j RunJob, status string, duration time.Duration) junit.TestSuites {
	tc := junit.TestCase{
		Name:      c.TestCase.Name,
		ClassName: c.Suite.Name,
		Time:      fmt.Sprintf("%.0f", duration.Seconds()),
		Status:    status,
	}
	ts := junit.TestSuite{
		Name:  c.Suite.Name + " - " + c.TestCase.Name,
		Tests: 1,
		Time:  tc.Time,
	}

	switch status {
	case job.StateFailed:
		msg := j.Error
		if msg == "" {
			msg = "test case failed"
		}
		tc.Failure = &junit.Failure{Message: msg, Type: "failure", Text: msg}
		ts.Failures = 1
	case job.StateInProgress:
		msg := "the run did not finish before saucectl stopped waiting"
		tc.Error = &junit.Error{Message: msg, Type: "timeout", Text: msg}
		ts.Errors = 1
	}

	ts.TestCases = []junit.TestCase{tc}
	return junit.TestSuites{TestSuites: []junit.TestSuite{ts}}
}

// errString renders an error for a synthesized failure message.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
