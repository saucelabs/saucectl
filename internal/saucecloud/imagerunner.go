package saucecloud

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/ryanuber/go-glob"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/report"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/imagerunner"
)

type ImageRunner interface {
	TriggerRun(context.Context, imagerunner.RunnerSpec) (imagerunner.Runner, error)
	GetStatus(ctx context.Context, id string) (imagerunner.Runner, error)
	StopRun(ctx context.Context, id string) error
	DownloadArtifacts(ctx context.Context, id string) (string, error)
	GetLogs(ctx context.Context, id string) (string, error)
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

func (r *ImgRunner) RunProject() (int, error) {
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

func (r *ImgRunner) runSuite(suite imagerunner.Suite) (imagerunner.Runner, error) {
	var run imagerunner.Runner
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

	var auth *imagerunner.Auth
	if suite.ImagePullAuth.User != "" && suite.ImagePullAuth.Token != "" {
		auth = &imagerunner.Auth{
			User:  suite.ImagePullAuth.User,
			Token: suite.ImagePullAuth.Token,
		}
	}

	runner, err := r.RunnerService.TriggerRun(ctx, imagerunner.RunnerSpec{
		Container: imagerunner.Container{
			Name: suite.Image,
			Auth: auth,
		},
		EntryPoint: suite.EntryPoint,
		Env:        mapEnv(suite.Env),
		Files:      files,
		Artifacts:  suite.Artifacts,
		Metadata:   metadata,
	})
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
		run.Status = imagerunner.StateCancelled
		return run, SuiteTimeoutError{Timeout: suite.Timeout}
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		run.Status = imagerunner.StateCancelled
		return run, ErrSuiteCancelled
	}
	if err != nil {
		return run, err
	}

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Str("runID", runner.ID).
		Msg("Started suite.")
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
		// TerminationReason is currently _not_ implemented on the server side. Conditional can be removed once
		// the server always sends back a response.
		err = fmt.Errorf("suite '%s' failed", suite.Name)
		if run.TerminationReason != "" {
			err = fmt.Errorf("suite '%s' failed: %s", suite.Name, run.TerminationReason)
		}
		return run, err
	}

	return run, err
}

func (r *ImgRunner) collectResults(results chan execResult, expected int) bool {
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

		r.PrintLogs(res.runID, res.name)

		r.DownloadArtifacts(res.runID, res.name, res.status, passed)

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

func extractFile(artifactFolder string, file *zip.File) error {
	fullPath := path.Join(artifactFolder, file.Name)

	relPath, err := filepath.Rel(artifactFolder, fullPath)
	if err != nil {
		return err
	}
	if strings.Contains(relPath, "..") {
		return fmt.Errorf("file %s is relative to an outside folder", file.Name)
	}

	folder := path.Dir(fullPath)
	if err := os.MkdirAll(folder, 0755); err != nil {
		return err
	}

	fd, err := os.Create(fullPath)
	if err != nil {
		return err
	}

	rd, err := file.Open()
	if err != nil {
		return err
	}
	_, err = io.Copy(fd, rd)
	if err != nil {
		return err
	}
	return nil
}

func (r *ImgRunner) DownloadArtifacts(runnerID, suiteName, status string, passed bool) {
	if runnerID == "" || status == imagerunner.StateCancelled || !r.Project.Artifacts.Download.When.IsNow(passed) {
		return
	}

	dir, err := config.GetSuiteArtifactFolder(suiteName, r.Project.Artifacts.Download)
	if err != nil {
		log.Err(err).Msg("Unable to create artifacts folder.")
		return
	}

	log.Info().Msg("Downloading artifacts archive")
	fileName, err := r.RunnerService.DownloadArtifacts(r.ctx, runnerID)
	if err != nil {
		log.Err(err).Str("suite", suiteName).Msg("Failed to fetch artifacts.")
		return
	}

	zf, err := zip.OpenReader(fileName)
	if err != nil {
		return
	}
	for _, f := range zf.File {
		for _, pattern := range r.Project.Artifacts.Download.Match {
			if glob.Glob(pattern, f.Name) {
				if err = extractFile(dir, f); err != nil {
					log.Error().Msgf("Unable to extract file '%s': %s", f.Name, err)
				}
				break
			}
		}
	}
}

func (r *ImgRunner) PrintLogs(runID, suiteName string) {
	if runID == "" {
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

// orDefault takes two values of type T and returns a if it's non-zero (not 0, "" etc.), b otherwise.
func orDefault[T comparable](a T, b T) T {
	if reflect.ValueOf(a).IsZero() {
		return b
	}

	return a
}
