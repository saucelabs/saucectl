package saucecloud

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/ryanuber/go-glob"

	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader storage.ProjectUploader
	JobStarter      job.Starter
	JobReader       job.Reader
	JobStopper      job.Stopper
	CCYReader       concurrency.Reader
	TunnelService   tunnel.Service
	Region          region.Region
	ShowConsoleLog  bool

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

func (r *CloudRunner) createWorkerPool(num int) (chan job.StartOptions, chan result) {
	jobOpts := make(chan job.StartOptions)
	results := make(chan result, num)

	ccy := concurrency.Min(r.CCYReader, num)
	log.Info().Int("concurrency", ccy).Msg("Launching workers.")
	for i := 0; i < ccy; i++ {
		go r.runJobs(jobOpts, results)
	}

	return jobOpts, results
}

func (r *CloudRunner) collectResults(artifactsCfg config.ArtifactDownload, results chan result, expected int) bool {
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
				break
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

		r.downloadArtifacts(artifactsCfg, res.job)
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

	// os.Interrupt can arrive before the signal.Notify() is registered. In that case,
	// if a soft exit is requested during startContainer phase, it gently exits.
	if r.interrupted {
		return job.Job{}, true, nil
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), id)
	log.Info().Str("suite", opts.Suite).Str("url", jobDetailsPage).Msg("Suite started.")

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
	assetContent, err := r.JobReader.GetJobAssetFileContent(context.Background(), res.job.ID, ConsoleLogAsset)
	if err != nil {
		log.Warn().Str("suite", res.suiteName).Msg("Failed to retrieve the console output.")
	} else {
		log.Info().Str("suite", res.suiteName).Msgf("console.log output: \n%s", assetContent)
	}
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

// downloadArtifacts downloads artifact for job.
func (r *CloudRunner) downloadArtifacts(artifactsCfg config.ArtifactDownload, jb job.Job) {
	if !r.shouldDownloadArtifacts(artifactsCfg, jb) {
		return
	}

	targetDir := filepath.Join(artifactsCfg.Directory, jb.ID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Error().Msgf("Unable to create %s to fetch artifacts (%v)", targetDir, err)
		return
	}

	files, err := r.JobReader.GetJobAssetFileNames(context.Background(), jb.ID)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return
	}
	for _, fileName := range files {
		for _, pattern := range artifactsCfg.Match {
			if glob.Glob(pattern, fileName) {
				if err := r.doDownloadArtifact(targetDir, jb.ID, fileName); err != nil {
					log.Error().Err(err).Msgf("Failed to download file: %s", fileName)
				}
				break
			}
		}
	}
}

func (r *CloudRunner) doDownloadArtifact(targetDir, jobID, fileName string) error {
	fileContent, err := r.JobReader.GetJobAssetFileContent(context.Background(), jobID, fileName)
	if err != nil {
		return err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return os.WriteFile(targetFile, fileContent, 0644)
}

func (r *CloudRunner) shouldDownloadArtifacts(artifactsCfg config.ArtifactDownload, jb job.Job) bool {
	if jb.ID == "" {
		return false
	}
	if artifactsCfg.When == config.WhenAlways {
		return true
	}
	if artifactsCfg.When == config.WhenFail && jb.Status == job.StateError {
		return true
	}
	if artifactsCfg.When == config.WhenPass && jb.Status == job.StateComplete {
		return true
	}
	return false
}
