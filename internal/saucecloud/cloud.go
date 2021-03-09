package saucecloud

import (
	"context"
	"fmt"
	"github.com/saucelabs/saucectl/internal/tunnel"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/storage"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader storage.ProjectUploader
	JobStarter      job.Starter
	JobReader       job.Reader
	CCYReader       concurrency.Reader
	TunnelService   tunnel.Service
	Region          region.Region
	ShowConsoleLog  bool
}

type result struct {
	suiteName string
	browser   string
	job       job.Job
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

func (r *CloudRunner) collectResults(results chan result, expected int) bool {
	// TODO find a better way to get the expected
	errCount := 0
	completed := 0
	inProgress := expected
	passed := true

	done := make(chan interface{})
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				break
			case <-t.C:
				log.Info().Msgf("Suites completed: %d/%d", completed, expected)
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

		r.logSuite(res)

		if res.job.ID == "" || res.err != nil {
			errCount++
		}
	}
	close(done)

	log.Info().Msgf("Suites expected: %d", expected)
	log.Info().Msgf("Suites passed: %d", expected-errCount)
	log.Info().Msgf("Suites failed: %d", errCount)

	return passed
}

func (r *CloudRunner) runJob(opts job.StartOptions) (job.Job, error) {
	log.Info().Str("suite", opts.Suite).Str("region", r.Region.String()).Msg("Starting suite.")

	id, err := r.JobStarter.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{}, err
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), id)
	log.Info().Str("suite", opts.Suite).Str("url", jobDetailsPage).Msg("Suite started.")

	// High interval poll to not oversaturate the job reader with requests.
	j, err := r.JobReader.PollJob(context.Background(), id, 15*time.Second)
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to retrieve job status for suite %s", opts.Suite)
	}

	if !j.Passed {
		// We may need to differentiate when a job has crashed vs. when there is errors.
		return j, fmt.Errorf("suite '%s' has test failures", opts.Suite)
	}

	return j, nil
}

func (r *CloudRunner) runJobs(jobOpts <-chan job.StartOptions, results chan<- result) {
	for opts := range jobOpts {
		jobData, err := r.runJob(opts)

		results <- result{
			suiteName: opts.Suite,
			browser:   opts.BrowserName,
			job:       jobData,
			err:       err,
		}
	}
}

func (r CloudRunner) archiveAndUpload(project interface{}, files []string, sauceignoreFile string) (string, error) {
	tempDir, err := ioutil.TempDir(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(project, tempDir, files, sauceignoreFile)
	if err != nil {
		return "", err
	}

	return r.uploadProject(zipName)
}

func (r *CloudRunner) archiveProject(project interface{}, tempDir string, files []string, sauceignoreFile string) (string, error) {
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
	files = append(files, rcPath)

	for _, f := range files {
		if err := z.Add(f, ""); err != nil {
			return "", err
		}
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

func (r *CloudRunner) uploadProject(filename string) (string, error) {
	progress.Show("Uploading project")
	start := time.Now()
	resp, err := r.ProjectUploader.Upload(filename)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Dur("durationMs", time.Since(start)).Str("storageId", resp.ID).Msg("Project uploaded.")
	return resp.ID, nil
}

// logSuite display the result of a suite
func (r *CloudRunner) logSuite(res result) {
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
