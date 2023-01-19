package saucecloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/saucelabs/saucectl/internal/report"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/hostedexec"
)

type state uint8

const (
	running state = iota
	stopping
)

type HostedExecRunner struct {
	Project       hostedexec.Project
	RunnerService hostedexec.Service
	state         state

	Reporters []report.Reporter
}

type execResult struct {
	name      string
	skipped   bool
	status    string
	err       error
	duration  time.Duration
	startTime time.Time
	endTime   time.Time
}

func (r *HostedExecRunner) RunProject() (int, error) {
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

		run, err := r.runSuite(suite)
		if err != nil {
			log.Warn().Err(err).Msgf("Suite errored.")
		}

		results <- execResult{
			name:      suite.Name,
			status:    run.Status,
			err:       err,
			startTime: startTime,
			endTime:   time.Now(),
			duration:  time.Since(startTime),
		}
	}
}

func (r *HostedExecRunner) runSuite(suite hostedexec.Suite) (hostedexec.RunnerDetails, error) {
	var run hostedexec.RunnerDetails
	metadata := make(map[string]string)
	metadata["name"] = suite.Name

	files, err := mapFiles(suite.Files)
	if err != nil {
		log.Err(err).Str("suite", suite.Name).Msg("Unable to read source files")
		return run, err
	}

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Msg("Starting suite.")

	runner, err := r.RunnerService.TriggerRun(context.Background(), hostedexec.RunnerSpec{
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
		Metadata:   metadata,
	})
	if err != nil {
		return run, err
	}

	sigChan := r.registerInterruptOnSignal(runner.ID, suite.Name)
	defer unregisterSignalCapture(sigChan)

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Msg("Started suite.")
	run, err = r.PollRun(context.Background(), runner.ID)
	if err != nil {
		return run, err
	}

	// TODO: What's the failed status for a runner?
	if run.Status != hostedexec.StateSucceeded {
		return run, fmt.Errorf("suite '%s' failed", suite.Name)
	}

	return run, nil
}

func (r *HostedExecRunner) collectResults(results chan execResult, expected int) bool {
	inProgress := expected
	passed := true

	done := make(chan interface{})
	go func(r *HostedExecRunner) {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if r.state == running {
					log.Info().Msgf("Suites in progress: %d", inProgress)
				}
			}
		}
	}(r)
	for i := 0; i < expected; i++ {
		res := <-results
		inProgress--

		if res.err != nil {
			passed = false
		}

		for _, r := range r.Reporters {
			r.Add(report.TestResult{
				Name:      res.name,
				Duration:  res.duration,
				StartTime: res.startTime,
				EndTime:   res.endTime,
				Status:    res.status,
				Attempts:  1,
			})
		}
	}
	close(done)

	for _, r := range r.Reporters {
		r.Render()
	}

	return passed
}

func (r *HostedExecRunner) registerInterruptOnSignal(runID string, suiteName string) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, runID string, hr *HostedExecRunner) {
		for {
			sig := <-c
			if sig == nil {
				return
			}
			switch hr.state {
			case running:
				log.Info().Str("suite", suiteName).Msg("Stopping suite")
				err := hr.RunnerService.StopRun(context.Background(), runID)
				if err != nil {
					log.Warn().Err(err).Str("suite", suiteName).Msg("Unable to stop suite.")
				}
				println("\nStopping run. Waiting for all tests in progress to be stopped... (press Ctrl-c again to exit without waiting)\n")
				hr.state = stopping
			case stopping:
				os.Exit(1)
			}
		}
	}(sigChan, runID, r)
	return sigChan
}

func (r *HostedExecRunner) PollRun(ctx context.Context, id string) (hostedexec.RunnerDetails, error) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	deathclock := time.NewTimer(24 * time.Hour)
	defer deathclock.Stop()

	for {
		select {
		case <-ticker.C:
			r, err := r.RunnerService.GetRun(ctx, id)
			if err != nil {
				return hostedexec.RunnerDetails{}, err
			}
			if hostedexec.Done(r.Status) {
				return r, nil
			}
		case <-deathclock.C:
			r, err := r.RunnerService.GetRun(ctx, id)
			if err != nil {
				return hostedexec.RunnerDetails{}, err
			}
			return r, nil
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
