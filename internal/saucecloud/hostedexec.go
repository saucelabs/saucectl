package saucecloud

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/report"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/hostedexec"
)

type ImageRunner interface {
	TriggerRun(context.Context, hostedexec.RunnerSpec) (hostedexec.Runner, error)
	GetStatus(ctx context.Context, id string) (hostedexec.RunnerStatus, error)
	StopRun(ctx context.Context, id string) error
}

type SuiteTimeoutError struct {
	Timeout time.Duration
}

func (s SuiteTimeoutError) Error() string {
	return fmt.Sprintf("suite timed out after %s", s.Timeout)
}

var ErrSuiteCancelled = errors.New("suite cancelled")

type HostedExecRunner struct {
	Project       hostedexec.Project
	RunnerService ImageRunner

	Reporters []report.Reporter

	ctx    context.Context
	cancel context.CancelFunc
}

type execResult struct {
	name      string
	runID     string
	status    string
	err       error
	duration  time.Duration
	startTime time.Time
	endTime   time.Time
	attempts  int
}

func (r *HostedExecRunner) RunProject() (int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	r.cancel = cancel

	sigChan := r.registerInterruptOnSignal()
	defer unregisterSignalCapture(sigChan)

	suites, results := r.createWorkerPool(1, 0)

	// Submit suites to work on.
	go func() {
		for _, s := range r.Project.Suites {
			suites <- s
		}
	}()

	if passed := r.collectResults(results, len(r.Project.Suites)); !passed {
		return 1, nil
	}

	return 0, nil
}

func (r *HostedExecRunner) createWorkerPool(ccy int, maxRetries int) (chan hostedexec.Suite, chan execResult) {
	suites := make(chan hostedexec.Suite, maxRetries+1)
	results := make(chan execResult, ccy)

	log.Info().Int("concurrency", ccy).Msg("Launching workers.")
	for i := 0; i < ccy; i++ {
		go r.runSuites(suites, results)
	}

	return suites, results
}

func (r *HostedExecRunner) runSuites(suites chan hostedexec.Suite, results chan<- execResult) {
	for suite := range suites {
		startTime := time.Now()

		if r.ctx.Err() != nil {
			results <- execResult{
				name:      suite.Name,
				startTime: startTime,
				endTime:   time.Now(),
				duration:  time.Since(startTime),
				status:    hostedexec.StateCancelled,
				err:       ErrSuiteCancelled,
			}
			continue
		}

		run, err := r.runSuite(suite)

		results <- execResult{
			name:      suite.Name,
			runID:     run.ID,
			status:    run.Status,
			err:       err,
			startTime: startTime,
			endTime:   time.Now(),
			duration:  time.Since(startTime),
			attempts:  1,
		}
	}
}

func (r *HostedExecRunner) runSuite(suite hostedexec.Suite) (hostedexec.RunnerStatus, error) {
	var run hostedexec.RunnerStatus
	metadata := make(map[string]string)
	metadata["name"] = suite.Name

	files, err := mapFiles(suite.Files)
	if err != nil {
		log.Err(err).Str("suite", suite.Name).Msg("Unable to read source files")
		return run, err
	}

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Msg("Starting suite.")

	if suite.Timeout <= 0 {
		suite.Timeout = 24 * time.Hour
	}

	ctx, cancel := context.WithTimeout(r.ctx, suite.Timeout)
	defer cancel()

	runner, err := r.RunnerService.TriggerRun(ctx, hostedexec.RunnerSpec{
		Container: hostedexec.Container{
			Name: suite.Image,
			Auth: hostedexec.Auth{
				User:  suite.ImagePullAuth.User,
				Token: suite.ImagePullAuth.Token,
			},
		},
		EntryPoint: suite.EntryPoint,
		Env:        mapEnv(suite.Env),
		Files:      files,
		Artifacts:  suite.Artifacts,
		Metadata:   metadata,
	})
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
		run.Status = hostedexec.StateCancelled
		return run, SuiteTimeoutError{Timeout: suite.Timeout}
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		run.Status = hostedexec.StateCancelled
		return run, ErrSuiteCancelled
	}
	if err != nil {
		return run, err
	}

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Str("runID", runner.ID).
		Msg("Started suite.")
	run, err = r.PollRun(ctx, runner.ID)
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
		// Use a new context, because the suite's already timed out, and we'd not be able to stop the run.
		_ = r.RunnerService.StopRun(context.Background(), runner.ID)
		run.Status = hostedexec.StateCancelled
		return run, SuiteTimeoutError{Timeout: suite.Timeout}
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		// Use a new context, because saucectl is already interrupted, and we'd not be able to stop the run.
		_ = r.RunnerService.StopRun(context.Background(), runner.ID)
		run.Status = hostedexec.StateCancelled
		return run, ErrSuiteCancelled
	}
	if err != nil {
		return run, err
	}

	if run.Status != hostedexec.StateSucceeded {
		return run, fmt.Errorf("suite '%s' failed", suite.Name)
	}

	return run, nil
}

func (r *HostedExecRunner) collectResults(results chan execResult, expected int) bool {
	inProgress := expected
	passed := true

	stopProgress := startProgressTicker(r.ctx, &inProgress)
	for i := 0; i < expected; i++ {
		res := <-results
		inProgress--

		if res.err != nil {
			passed = false
		}

		log.Err(res.err).Str("suite", res.name).Bool("passed", res.err == nil).Str("runID", res.runID).
			Msg("Suite finished.")

		for _, r := range r.Reporters {
			r.Add(report.TestResult{
				Name:      res.name,
				Duration:  res.duration,
				StartTime: res.startTime,
				EndTime:   res.endTime,
				Status:    res.status,
				Attempts:  res.attempts,
			})
		}
	}
	stopProgress()

	for _, r := range r.Reporters {
		r.Render()
	}

	return passed
}

func (r *HostedExecRunner) registerInterruptOnSignal() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, hr *HostedExecRunner) {
		for {
			sig := <-c
			if sig == nil {
				return
			}
			if r.ctx.Err() == nil {
				r.cancel()
				println("\nStopping run. Cancelling all suites in progress... (press Ctrl-c again to exit without waiting)\n")
			} else {
				os.Exit(1)
			}
		}
	}(sigChan, r)
	return sigChan
}

func (r *HostedExecRunner) PollRun(ctx context.Context, id string) (hostedexec.RunnerStatus, error) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return hostedexec.RunnerStatus{}, ctx.Err()
		case <-ticker.C:
			r, err := r.RunnerService.GetStatus(ctx, id)
			if err != nil || hostedexec.Done(r.Status) {
				return r, err
			}
		}
	}
}

func mapEnv(env map[string]string) []hostedexec.EnvItem {
	var items []hostedexec.EnvItem
	for key, val := range env {
		items = append(items, hostedexec.EnvItem{
			Name:  key,
			Value: val,
		})
	}
	return items
}

func mapFiles(files []hostedexec.File) ([]hostedexec.FileData, error) {
	var items []hostedexec.FileData
	for _, f := range files {
		data, err := readFile(f.Src)
		if err != nil {
			return items, err
		}
		items = append(items, hostedexec.FileData{
			Path: f.Dst,
			Data: data,
		})
	}
	return items, nil
}

func readFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.Strict().EncodeToString(bytes), nil
}

func startProgressTicker(ctx context.Context, progress *int) (cancel context.CancelFunc) {
	ctx, cancel = context.WithCancel(ctx)

	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				log.Info().Msgf("Suites in progress: %d", *progress)
			}
		}
	}()

	return
}
