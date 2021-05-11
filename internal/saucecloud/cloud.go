package saucecloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader    storage.ProjectUploader
	JobStarter         job.Starter
	JobReader          job.Reader
	JobWriter          job.Writer
	JobStopper         job.Stopper
	CCYReader          concurrency.Reader
	TunnelService      tunnel.Service
	Region             region.Region
	ShowConsoleLog     bool
	ArtifactDownloader download.ArtifactDownloader

	interrupted bool
}

type result struct {
	suiteName string
	browser   string
	job       job.Job
	skipped   bool
	err       error
}

// ConsoleLogAsset represents job asset log file name.
const ConsoleLogAsset = "console.log"

func (r *CloudRunner) createWorkerPool(num int) (chan job.StartOptions, chan result, error) {
	ccy := concurrency.Min(r.CCYReader, num)
	if ccy == 0 {
		log.Error().Msgf("No concurrency available")
		err := errors.New("no concurrency available")
		return nil, nil, err
	}

	jobOpts := make(chan job.StartOptions)
	results := make(chan result, num)

	log.Info().Int("concurrency", ccy).Msg("Launching workers.")
	for i := 0; i < ccy; i++ {
		go r.runJobs(jobOpts, results)
	}

	return jobOpts, results, nil
}

func (r *CloudRunner) collectResults(artifactCfg config.ArtifactDownload, results chan result, expected int) bool {
	// TODO find a better way to get the expected
	errCount := 0
	completed := 0
	inProgress := expected
	passed := true

	done := make(chan interface{})
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				log.Info().Msgf("Suites in progress: %d", inProgress)
			}
		}
	}()
	for i := 0; i < expected; i++ {
		res := <-results
		// in case one of test suites not passed
		if !res.job.Passed {
			passed = false
		}
		completed++
		inProgress--

		if download.ShouldDownloadArtifact(res.job.ID, res.job.Passed, artifactCfg) {
			r.ArtifactDownloader.DownloadArtifact(res.job.ID)
		}
		r.logSuite(res)

		if res.job.ID == "" || res.err != nil {
			errCount++
		}
	}
	close(done)

	if errCount != 0 {
		msg.LogTestFailure(errCount, expected)
		return passed
	}

	msg.LogTestSuccess()

	return passed
}

func (r *CloudRunner) runJob(opts job.StartOptions) (j job.Job, interrupted bool, err error) {
	log.Info().Str("suite", opts.Suite).Str("region", r.Region.String()).Msg("Starting suite.")

	id, err := r.JobStarter.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{}, false, err
	}

	r.uploadSauceConfig(id, opts.ConfigFilePath)

	// os.Interrupt can arrive before the signal.Notify() is registered. In that case,
	// if a soft exit is requested during startContainer phase, it gently exits.
	if r.interrupted {
		return job.Job{}, true, nil
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), id)
	l := log.Info().Str("url", jobDetailsPage).Str("suite", opts.Suite).Str("platform", opts.PlatformName)
	if opts.Framework == config.KindEspresso {
		l.Str("device", opts.DeviceName).Str("name", opts.Name).Str("platformVersion", opts.PlatformVersion)

	} else {
		l.Str("browser", opts.BrowserName)
	}
	l.Msg("Suite started.")

	sigChan := r.registerInterruptOnSignal(id, opts.Suite)
	defer unregisterSignalCapture(sigChan)

	// High interval poll to not oversaturate the job reader with requests.
	j, err = r.JobReader.PollJob(context.Background(), id, 15*time.Second)
	if err != nil {
		return job.Job{}, false, fmt.Errorf("failed to retrieve job status for suite %s", opts.Suite)
	}

	if !j.Passed {
		// We may need to differentiate when a job has crashed vs. when there is errors.
		return j, false, fmt.Errorf("suite '%s' has test failures", opts.Suite)
	}

	return j, false, nil
}

func (r *CloudRunner) runJobs(jobOpts <-chan job.StartOptions, results chan<- result) {
	for opts := range jobOpts {
		if r.interrupted {
			results <- result{
				suiteName: opts.Suite,
				browser:   opts.BrowserName,
				skipped:   true,
				err:       nil,
			}
			continue
		}
		jobData, skipped, err := r.runJob(opts)

		results <- result{
			suiteName: opts.Suite,
			browser:   opts.BrowserName,
			job:       jobData,
			skipped:   skipped,
			err:       err,
		}
	}
}

func (r CloudRunner) archiveAndUpload(project interface{}, folder string, sauceignoreFile string) (string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(project, tempDir, folder, sauceignoreFile)
	if err != nil {
		return "", err
	}

	return r.uploadProject(zipName, projectUpload)
}

func (r *CloudRunner) archiveProject(project interface{}, tempDir string, projectFolder string, sauceignoreFile string) (string, error) {
	start := time.Now()

	matcher, err := sauceignore.NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		return "", err
	}

	zipName := filepath.Join(tempDir, "app.zip")
	z, err := zip.NewWriter(zipName, matcher)
	if err != nil {
		return "", err
	}
	defer z.Close()

	rcPath := filepath.Join(tempDir, "sauce-runner.json")
	if err := jsonio.WriteFile(rcPath, project); err != nil {
		return "", err
	}

	folderContent, err := os.ReadDir(projectFolder)
	if err != nil {
		return "", err
	}

	for _, child := range folderContent {
		log.Debug().Str("name", child.Name()).Msg("Adding to archive")
		if err := z.Add(filepath.Join(projectFolder, child.Name()), ""); err != nil {
			return "", err
		}
	}
	log.Debug().Str("name", rcPath).Msg("Adding to archive")
	if err := z.Add(rcPath, ""); err != nil {
		return "", err
	}

	err = z.Close()
	if err != nil {
		return "", err
	}

	f, err := os.Stat(zipName)
	if err != nil {
		return "", err
	}

	log.Info().Dur("durationMs", time.Since(start)).Int64("size", f.Size()).Msg("Project archived.")

	return zipName, nil
}

type uploadType string

var (
	testAppUpload uploadType = "test application"
	appUpload     uploadType = "application"
	projectUpload uploadType = "project"
)

func (r *CloudRunner) uploadProject(filename string, pType uploadType) (string, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		return "", nil
	}
	progress.Show("Uploading %s %s", pType, filename)

	start := time.Now()
	resp, err := r.ProjectUploader.Upload(filename)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Dur("durationMs", time.Since(start)).Str("storageId", resp.ID).Msgf("%s uploaded.", strings.Title(string(pType)))
	return resp.ID, nil
}

// logSuite display the result of a suite
func (r *CloudRunner) logSuite(res result) {
	if res.skipped {
		log.Error().Err(res.err).Str("suite", res.suiteName).Msg("Suite skipped.")
		return
	}
	if res.job.ID == "" {
		log.Error().Err(res.err).Str("suite", res.suiteName).Msg("Failed to start suite.")
		return
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), res.job.ID)
	if res.job.Passed {
		log.Info().Str("suite", res.suiteName).Bool("passed", res.job.Passed).Str("url", jobDetailsPage).
			Msg("Suite finished.")
	} else {
		log.Error().Str("suite", res.suiteName).Bool("passed", res.job.Passed).Str("url", jobDetailsPage).
			Msg("Suite finished.")
	}
	r.logSuiteConsole(res)
}

// logSuiteError display the console output when tests from a suite are failing
func (r *CloudRunner) logSuiteConsole(res result) {
	// To avoid clutter, we don't show the console on job passes.
	if res.job.Passed && !r.ShowConsoleLog {
		return
	}

	// Display log only when at least it has started
	var assetContent []byte
	var err error
	if assetContent, err = r.JobReader.GetJobAssetFileContent(context.Background(), res.job.ID, ConsoleLogAsset); err == nil {
		log.Info().Str("suite", res.suiteName).Msgf("console.log output: \n%s", assetContent)
		return
	}

	// Some frameworks produce a junit.xml instead, check for that file if there's no console.log
	assetContent, err = r.JobReader.GetJobAssetFileContent(context.Background(), res.job.ID, "junit.xml")
	if err != nil {
		log.Warn().Str("suite", res.suiteName).Msg("Failed to retrieve the console output.")
		return
	}

	var testsuites junit.TestSuites
	if testsuites, err = junit.Parse(assetContent); err != nil {
		log.Warn().Str("suite", res.suiteName).Msg("Failed to parse junit")
		return
	}

	// Print summary of failures from junit.xml
	headerColor := color.New(color.FgRed).Add(color.Bold).Add(color.Underline)
	headerColor.Print("\nErrors:\n\n")
	bodyColor := color.New(color.FgHiRed)
	errCount := 1
	for _, ts := range testsuites.TestSuite {
		for _, tc := range ts.TestCase {
			if tc.Error != "" {
				fmt.Printf("\t%d) %s.%s\n\n", errCount, tc.ClassName, tc.Name)
				headerColor.Println("\tError was:")
				bodyColor.Printf("\t%s\n", tc.Error)
				errCount++
			}
		}
	}

	// Print summary table
	fmt.Println()
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"espresso testsuite", "tests", "pass", "fail", "error"})
	for _, ts := range testsuites.TestSuite {
		passed := ts.Tests - ts.Errors - ts.Failures
		t.AppendRow(table.Row{ts.Package, ts.Tests, passed, ts.Failures, ts.Errors})
	}
	t.Render()
	fmt.Println()
}

func (r *CloudRunner) validateTunnel(id string) error {
	if id == "" {
		return nil
	}

	// This wait value is deliberately not configurable.
	wait := 30 * time.Second
	log.Info().Str("timeout", wait.String()).Msg("Performing tunnel readiness check...")
	if err := r.TunnelService.IsTunnelRunning(context.Background(), id, wait); err != nil {
		return err
	}

	log.Info().Msg("Tunnel is ready!")
	return nil
}

func (r *CloudRunner) dryRun(project interface{}, folder string, sauceIgnoreFile string, suiteNames string) error {
	log.Warn().Msg("Running tests in dry run mode.")
	tmpDir, err := os.MkdirTemp("./", "sauce-app-payload-*")
	if err != nil {
		return err
	}
	log.Info().Msgf("The following test suites would have run: [%s].", suiteNames)
	zipName, err := r.archiveProject(project, tmpDir, folder, sauceIgnoreFile)
	if err != nil {
		return err
	}

	log.Info().Msgf("Saving bundled project to %s.", zipName)
	return nil
}

// stopSuiteExecution stops the current execution on Sauce Cloud
func (r *CloudRunner) stopSuiteExecution(jobID string, suiteName string) {
	_, err := r.JobStopper.StopJob(context.Background(), jobID)
	if err != nil {
		log.Warn().Err(err).Str("suite", suiteName).Msg("Unable to stop suite.")
	}
}

// registerInterruptOnSignal stops execution on Sauce Cloud when a SIGINT is captured.
func (r *CloudRunner) registerInterruptOnSignal(jobID, suiteName string) chan os.Signal {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, jobID, suiteName string) {
		sig := <-c
		if sig == nil {
			return
		}
		log.Info().Str("suite", suiteName).Msg("Stopping suite")
		r.stopSuiteExecution(jobID, suiteName)
	}(sigChan, jobID, suiteName)
	return sigChan
}

// registerSkipSuitesOnSignal prevent new suites from being executed when a SIGINT is captured.
func (r *CloudRunner) registerSkipSuitesOnSignal() chan os.Signal {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, cr *CloudRunner) {
		for {
			sig := <-c
			if sig == nil {
				return
			}
			if cr.interrupted {
				os.Exit(1)
			}
			log.Info().Msg("Ctrl-C captured. Ctrl-C again to exit now.")
			cr.interrupted = true
		}
	}(sigChan, r)
	return sigChan
}

// unregisterSignalCapture remove the signal hook associated to the chan c.
func unregisterSignalCapture(c chan os.Signal) {
	signal.Stop(c)
	close(c)
}

// uploadSauceConfig adds job configuration as an asset.
func (r *CloudRunner) uploadSauceConfig(jobID string, cfgFile string) {
	f, err := os.Open(cfgFile)
	if err != nil {
		log.Warn().Msgf("failed to open configuration: %v", err)
		return
	}
	content, err := io.ReadAll(f)
	if err != nil {
		log.Warn().Msgf("failed to read configuration: %v", err)
		return
	}
	if err := r.JobWriter.UploadAsset(jobID, filepath.Base(cfgFile), "text/plain", content); err != nil {
		log.Warn().Msgf("failed to attach configuration: %v", err)
	}
}
