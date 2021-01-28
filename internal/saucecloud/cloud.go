package saucecloud

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/dots"
	"github.com/saucelabs/saucectl/cli/progress"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/storage"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader storage.ProjectUploader
	JobStarter      job.Starter
	JobReader       job.Reader
	CCYReader       concurrency.Reader
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

	waiter := dots.New(1)
	waiter.Start()
	for i := 0; i < expected; i++ {
		res := <-results
		// in case one of test suites not passed
		if !res.job.Passed {
			passed = false
		}
		completed++
		inProgress--

		// Logging is not synchronized over the different worker routines & dot routine.
		// To avoid implementing a more complex solution centralizing output on only one
		// routine, a new lines has simply been forced, to ensure that line starts from
		// the beginning of the console.
		fmt.Println("")
		log.Info().Msgf("Suites completed: %d/%d", completed, expected)
		r.logSuite(res)

		if res.job.ID == "" || res.err != nil {
			errCount++
		}
	}
	waiter.Stop()

	log.Info().Msgf("Suites expected: %d", expected)
	log.Info().Msgf("Suites passed: %d", expected-errCount)
	log.Info().Msgf("Suites failed: %d", errCount)

	return passed
}

func (r *CloudRunner) runJob(opts job.StartOptions) (job.Job, error) {
	log.Info().Str("suite", opts.Suite).Str("region", r.Region.String()).Msg("Starting job.")

	id, err := r.JobStarter.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{}, err
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), id)
	log.Info().Msg(fmt.Sprintf("Job started - %s", jobDetailsPage))

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

func (r CloudRunner) archiveAndUpload(project interface{}, files []string) (string, error) {
	tempDir, err := ioutil.TempDir(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(project, tempDir, files)
	if err != nil {
		return "", err
	}

	return r.uploadProject(zipName)
}

func (r *CloudRunner) archiveProject(project interface{}, tempDir string, files []string) (string, error) {
	zipName := filepath.Join(tempDir, "app.zip")
	z, err := zip.NewWriter(zipName)
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

	log.Info().Int64("bytes", f.Size()).Msg("Project archived.")

	return zipName, nil
}

func (r *CloudRunner) uploadProject(filename string) (string, error) {
	progress.Show("Uploading project")
	resp, err := r.ProjectUploader.Upload(filename)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Str("storageId", resp.ID).Msg("Project uploaded.")
	return resp.ID, nil
}

// logSuite display the result of a suite
func (r *CloudRunner) logSuite(res result) {
	if res.job.ID == "" {
		log.Error().Str("suite", res.suiteName).Msgf("failed to start")
		log.Error().Str("suite", res.suiteName).Msgf("%s", res.err)
		return
	}
	resultStr := "Passed"
	if !res.job.Passed {
		resultStr = "Failed"
	}
	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), res.job.ID)
	log.Info().Str("suite", res.suiteName).Msgf("Status: %s - %s", resultStr, jobDetailsPage)
	r.logSuiteConsole(res)
}

// logSuiteError display the console output when tests from a suite are failing
func (r *CloudRunner) logSuiteConsole(res result) {
	// To avoid clutter, we don't show the console on job passes.
	if res.job.Passed || !r.ShowConsoleLog {
		return
	}

	// Display log only when at least it has started
	assetContent, err := r.JobReader.GetJobAssetFileContent(context.Background(), res.job.ID, ConsoleLogAsset)
	if err != nil {
		log.Warn().Str("suite", res.suiteName).Msg("Failed to get job asset.")
	} else {
		log.Info().Msg(fmt.Sprintf("Test %s %s", res.job.ID, ConsoleLogAsset))
		log.Info().Msg(string(assetContent))
	}
}
