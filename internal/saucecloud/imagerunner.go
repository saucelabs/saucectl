package saucecloud

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/ryanuber/go-glob"
	szip "github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fileio"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

type ImageRunner interface {
	TriggerRun(context.Context, imagerunner.RunnerSpec) (imagerunner.Runner, error)
	GetStatus(ctx context.Context, id string) (imagerunner.Runner, error)
	StopRun(ctx context.Context, id string) error
	DownloadArtifacts(ctx context.Context, id string) (io.ReadCloser, error)
	GetLogs(ctx context.Context, id string) (string, error)
	StreamLiveLogs(ctx context.Context, id string, wait bool) error
	GetLiveLogs(ctx context.Context, id string) error
}

type SuiteTimeoutError struct {
	Timeout time.Duration
}

func (s SuiteTimeoutError) Error() string {
	return fmt.Sprintf("suite timed out after %s", s.Timeout)
}

var ErrSuiteCancelled = errors.New("suite cancelled")

type ImgRunner struct {
	Project       imagerunner.Project
	RunnerService ImageRunner
	TunnelService tunnel.Service

	Reporters []report.Reporter

	Async             bool
	AsyncEventManager imagerunner.AsyncEventManager

	ctx    context.Context
	cancel context.CancelFunc
}

func NewImgRunner(project imagerunner.Project, runnerService ImageRunner, tunnelService tunnel.Service,
	asyncEventManager imagerunner.AsyncEventManager, reporters []report.Reporter, async bool) *ImgRunner {
	return &ImgRunner{
		Project:           project,
		RunnerService:     runnerService,
		TunnelService:     tunnelService,
		Reporters:         reporters,
		Async:             async,
		AsyncEventManager: asyncEventManager,
	}
}

type execResult struct {
	name      string
	runID     string
	status    string
	err       error
	duration  time.Duration
	startTime time.Time
	endTime   time.Time
	attempts  []report.Attempt
}

func (r *ImgRunner) RunProject() (int, error) {
	if err := tunnel.ValidateTunnel(r.TunnelService, r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner, tunnel.NoneFilter, false); err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		printDryRunSuiteNames(r.getSuiteNames())
		return 0, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	r.cancel = cancel

	sigChan := r.registerInterruptOnSignal()
	defer unregisterSignalCapture(sigChan)

	suites, results := r.createWorkerPool(r.Project.Sauce.Concurrency, 0)

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

func (r *ImgRunner) createWorkerPool(ccy int, maxRetries int) (chan imagerunner.Suite, chan execResult) {
	suites := make(chan imagerunner.Suite, maxRetries+1)
	results := make(chan execResult, ccy)

	log.Info().Int("concurrency", ccy).Msg("Launching workers.")
	for i := 0; i < ccy; i++ {
		go r.runSuites(suites, results)
	}

	return suites, results
}

func (r *ImgRunner) runSuites(suites chan imagerunner.Suite, results chan<- execResult) {
	for suite := range suites {
		// Apply defaults.
		defaults := r.Project.Defaults
		if defaults.Name != "" {
			suite.Name = defaults.Name + " " + suite.Name
		}

		suite.Image = orDefault(suite.Image, defaults.Image)
		suite.ImagePullAuth = orDefault(suite.ImagePullAuth, defaults.ImagePullAuth)
		suite.EntryPoint = orDefault(suite.EntryPoint, defaults.EntryPoint)
		suite.Timeout = orDefault(suite.Timeout, defaults.Timeout)
		suite.Files = append(suite.Files, defaults.Files...)
		suite.Artifacts = append(suite.Artifacts, defaults.Artifacts...)

		if suite.Env == nil {
			suite.Env = make(map[string]string)
		}
		for k, v := range defaults.Env {
			suite.Env[k] = v
		}

		startTime := time.Now()

		if r.ctx.Err() != nil {
			results <- execResult{
				name:      suite.Name,
				startTime: startTime,
				endTime:   time.Now(),
				duration:  time.Since(startTime),
				status:    imagerunner.StateCancelled,
				err:       ErrSuiteCancelled,
			}
			continue
		}

		run, err := r.runSuite(suite)

		endTime := time.Now()
		duration := time.Since(startTime)

		results <- execResult{
			name:      suite.Name,
			runID:     run.ID,
			status:    run.Status,
			err:       err,
			startTime: startTime,
			endTime:   endTime,
			duration:  duration,
			attempts: []report.Attempt{{
				ID:        run.ID,
				Duration:  duration,
				StartTime: startTime,
				EndTime:   endTime,
				Status:    run.Status,
			}},
		}
	}
}

func (r *ImgRunner) buildService(serviceIn imagerunner.SuiteService, suiteName string) (imagerunner.Service, error) {
	var auth *imagerunner.Auth
	if serviceIn.ImagePullAuth.User != "" && serviceIn.ImagePullAuth.Token != "" {
		auth = &imagerunner.Auth{
			User:  serviceIn.ImagePullAuth.User,
			Token: serviceIn.ImagePullAuth.Token,
		}
	}

	files, err := mapFiles(serviceIn.Files)
	if err != nil {
		log.Err(err).Str("suite", suiteName).Str("service", serviceIn.Name).Msg("Unable to read source files")
		return imagerunner.Service{}, err
	}

	serviceOut := imagerunner.Service{
		Name: serviceIn.Name,
		Container: imagerunner.Container{
			Name: serviceIn.Image,
			Auth: auth,
		},

		EntryPoint: serviceIn.EntryPoint,
		Env:        mapEnv(serviceIn.Env),
		Files:      files,
	}
	return serviceOut, nil
}

func (r *ImgRunner) runSuite(suite imagerunner.Suite) (imagerunner.Runner, error) {
	files, err := mapFiles(suite.Files)
	if err != nil {
		log.Err(err).Str("suite", suite.Name).Msg("Unable to read source files")
		return imagerunner.Runner{}, err
	}

	log.Info().
		Str("image", suite.Image).
		Str("suite", suite.Name).
		Str("tunnel", r.Project.Sauce.Tunnel.Name).
		Msg("Starting suite.")

	if suite.Timeout <= 0 {
		suite.Timeout = 24 * time.Hour
	}

	ctx, cancel := context.WithTimeout(r.ctx, suite.Timeout)
	defer cancel()

	var auth *imagerunner.Auth
	if suite.ImagePullAuth.User != "" && suite.ImagePullAuth.Token != "" {
		auth = &imagerunner.Auth{
			User:  suite.ImagePullAuth.User,
			Token: suite.ImagePullAuth.Token,
		}
	}

	services := make([]imagerunner.Service, len(suite.Services))
	for i, s := range suite.Services {
		services[i], err = r.buildService(s, suite.Name)
		if err != nil {
			return imagerunner.Runner{}, err
		}
	}

	runner, err := r.RunnerService.TriggerRun(ctx, imagerunner.RunnerSpec{
		Container: imagerunner.Container{
			Name: suite.Image,
			Auth: auth,
		},

		EntryPoint:   suite.EntryPoint,
		Env:          mapEnv(suite.Env),
		Files:        files,
		Artifacts:    suite.Artifacts,
		Metadata:     suite.Metadata,
		WorkloadType: suite.Workload,
		Tunnel:       r.getTunnel(),
		Services:     services,
	})

	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
		runner.Status = imagerunner.StateCancelled
		return runner, SuiteTimeoutError{Timeout: suite.Timeout}
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		runner.Status = imagerunner.StateCancelled
		return runner, ErrSuiteCancelled
	}
	if err != nil {
		runner.Status = imagerunner.StateFailed
		return runner, err
	}

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Str("runID", runner.ID).
		Msg("Started suite.")

	if r.Async {
		// Async mode means we don't wait for the suite to finish.
		return runner, nil
	}

	go r.pollLiveLogs(ctx, runner)

	var run imagerunner.Runner
	run, err = r.PollRun(ctx, runner.ID, runner.Status)
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
		// Use a new context, because the suite's already timed out, and we'd not be able to stop the run.
		_ = r.RunnerService.StopRun(context.Background(), runner.ID)
		run.Status = imagerunner.StateCancelled
		return run, SuiteTimeoutError{Timeout: suite.Timeout}
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		// Use a new context, because saucectl is already interrupted, and we'd not be able to stop the run.
		_ = r.RunnerService.StopRun(context.Background(), runner.ID)
		run.Status = imagerunner.StateCancelled
		return run, ErrSuiteCancelled
	}
	if err != nil {
		return run, err
	}

	if run.Status != imagerunner.StateSucceeded {
		return run, fmt.Errorf("suite %q failed: %s", suite.Name, run.TerminationReason)
	}

	return run, err
}

func (r *ImgRunner) pollLiveLogs(ctx context.Context, runner imagerunner.Runner) {
	if !r.Project.LiveLogs {
		return
	}

	ignoreError := func(err error) bool {
		if err == nil {
			return true
		}
		if errors.Is(err, context.Canceled) {
			return true
		}
		if strings.Contains(err.Error(), "websocket: close") {
			return true
		}
		return false
	}

	err := r.RunnerService.StreamLiveLogs(ctx, runner.ID, true)
	if !ignoreError(err) {
		log.Err(err).Msg("Async event handler failed.")
	}
}

func (r *ImgRunner) getTunnel() *imagerunner.Tunnel {
	if r.Project.Sauce.Tunnel.Name == "" && r.Project.Sauce.Tunnel.Owner == "" {
		return nil
	}
	return &imagerunner.Tunnel{
		Name:  r.Project.Sauce.Tunnel.Name,
		Owner: r.Project.Sauce.Tunnel.Owner,
	}
}

func (r *ImgRunner) collectResults(results chan execResult, expected int) bool {
	inProgress := expected
	passed := true

	stopProgress := r.startProgressTicker(r.ctx, &inProgress)
	for i := 0; i < expected; i++ {
		res := <-results
		inProgress--

		if res.err != nil {
			passed = false
		}

		r.PrintResult(res)
		if !r.Project.LiveLogs {
			// only print logs if live logs are disabled
			r.PrintLogs(res.runID, res.name)
		}
		files := r.DownloadArtifacts(res.runID, res.name, res.status, res.err != nil)
		var artifacts []report.Artifact
		for _, f := range files {
			artifacts = append(artifacts, report.Artifact{FilePath: f})
		}

		for _, r := range r.Reporters {
			r.Add(report.TestResult{
				Name:      res.name,
				Duration:  res.duration,
				StartTime: res.startTime,
				EndTime:   res.endTime,
				Status:    res.status,
				Artifacts: artifacts,
				Platform:  "Linux",
				RunID:     res.runID,
				Attempts: []report.Attempt{{
					ID:        res.runID,
					Duration:  res.duration,
					StartTime: res.startTime,
					EndTime:   res.endTime,
					Status:    res.status,
				}},
			})
		}
	}
	stopProgress()

	for _, r := range r.Reporters {
		r.Render()
	}

	return passed
}

func (r *ImgRunner) registerInterruptOnSignal() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, hr *ImgRunner) {
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

func (r *ImgRunner) PollRun(ctx context.Context, id string, lastStatus string) (imagerunner.Runner, error) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return imagerunner.Runner{}, ctx.Err()
		case <-ticker.C:
			r, err := r.RunnerService.GetStatus(ctx, id)
			if err != nil {
				return r, err
			}
			if r.Status != lastStatus {
				log.Info().Str("runID", r.ID).Str("old", lastStatus).Str("new", r.Status).Msg("Status change.")
				lastStatus = r.Status
			}
			if imagerunner.Done(r.Status) {
				return r, err
			}
		}
	}
}

// DownloadArtifacts DownloadArtifact downloads a zipped archive of artifacts
// and extracts the required files.
func (r *ImgRunner) DownloadArtifacts(runnerID, suiteName, status string, passed bool) []string {
	if r.Async ||
		runnerID == "" ||
		status == imagerunner.StateCancelled ||
		!r.Project.Artifacts.Download.When.IsNow(passed) {
		return nil
	}

	dir, err := config.GetSuiteArtifactFolder(suiteName, r.Project.Artifacts.Download)
	if err != nil {
		log.Err(err).Msg("Unable to create artifacts folder.")
		return nil
	}

	log.Info().Msg("Downloading artifacts archive")
	reader, err := r.RunnerService.DownloadArtifacts(r.ctx, runnerID)
	if err != nil {
		log.Err(err).Str("suite", suiteName).Msg("Failed to fetch artifacts.")
		return nil
	}
	defer reader.Close()

	fileName, err := fileio.CreateTemp(reader)
	if err != nil {
		log.Err(err).Str("suite", suiteName).Msg("Failed to download artifacts content.")
		return nil
	}
	defer os.Remove(fileName)

	zf, err := zip.OpenReader(fileName)
	if err != nil {
		log.Err(err).Msgf("Unable to open zip file %q", fileName)
		return nil
	}
	defer zf.Close()
	var artifacts []string
	for _, f := range zf.File {
		for _, pattern := range r.Project.Artifacts.Download.Match {
			if glob.Glob(pattern, f.Name) {
				if err = szip.Extract(dir, f); err != nil {
					log.Err(err).Msgf("Unable to extract file %q", f.Name)
				} else {
					artifacts = append(artifacts, filepath.Join(dir, f.Name))
				}
				break
			}
		}
	}
	return artifacts
}

func (r *ImgRunner) PrintResult(res execResult) {
	if r.Async {
		return
	}

	logEvent := log.Err(res.err).
		Str("suite", res.name).
		Bool("passed", res.err == nil).
		Str("runID", res.runID)

	if res.err != nil {
		logEvent.Msg("Suite failed.")
		return
	}

	logEvent.Msg("Suite finished.")
}

func (r *ImgRunner) PrintLogs(runID, suiteName string) {
	if r.Async || runID == "" {
		return
	}

	// Need a poll timeout, because artifacts may never exist.
	ctx, cancel := context.WithTimeout(r.ctx, 3*time.Minute)
	defer cancel()

	logs, err := r.PollLogs(ctx, runID)
	if err != nil {
		log.Err(err).Str("suite", suiteName).Msg("Unable to display logs.")
	} else {
		msg.LogConsoleOut(suiteName, logs)
	}
}

func (r *ImgRunner) PollLogs(ctx context.Context, id string) (string, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			l, err := r.RunnerService.GetLogs(ctx, id)
			if err == imagerunner.ErrResourceNotFound || errors.Is(err, context.DeadlineExceeded) {
				// Keep retrying on 404s or request timeouts. Might be available later.
				continue
			}
			return l, err
		}
	}
}

func (r *ImgRunner) getSuiteNames() []string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return names
}

func mapEnv(env map[string]string) []imagerunner.EnvItem {
	var items []imagerunner.EnvItem
	for key, val := range env {
		items = append(items, imagerunner.EnvItem{
			Name:  key,
			Value: val,
		})
	}
	return items
}

func mapFiles(files []imagerunner.File) ([]imagerunner.FileData, error) {
	var items []imagerunner.FileData
	for _, f := range files {
		data, err := readFile(f.Src)
		if err != nil {
			return items, err
		}
		items = append(items, imagerunner.FileData{
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

func (r *ImgRunner) startProgressTicker(ctx context.Context, progress *int) (cancel context.CancelFunc) {
	ctx, cancel = context.WithCancel(ctx)

	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if r.AsyncEventManager.IsLogIdle() {
					log.Info().Msgf("Suites in progress: %d", *progress)
				}
			}
		}
	}()

	return
}

// orDefault takes two values of type T and returns a if it's non-zero (not 0, "" etc.), b otherwise.
func orDefault[T comparable](a T, b T) T {
	if reflect.ValueOf(a).IsZero() {
		return b
	}

	return a
}
