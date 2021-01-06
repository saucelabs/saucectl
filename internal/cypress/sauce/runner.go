package sauce

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/cli/progress"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/storage"
)

// Runner represents the Sauce Labs cloud implementation for cypress.
type Runner struct {
	Project         cypress.Project
	ProjectUploader storage.ProjectUploader
	JobStarter      job.Starter
	JobReader       job.Reader
	Concurrency     int
	Region          region.Region
}

type result struct {
	suiteName string
	browser   string
	job       job.Job
	err       error
}

// RunProject runs the tests defined in cypress.Project.
func (r *Runner) RunProject() (int, error) {
	exitCode := 1

	err := r.JobStarter.CheckFrameworkAvailability(context.Background(), r.Project.Kind)
	if err != nil {
		err = fmt.Errorf("job pre-check failed; %s", err)
		return exitCode, err
	}

	// Archive the project files.
	tempDir, err := ioutil.TempDir(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return exitCode, err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(tempDir)
	if err != nil {
		return exitCode, err
	}

	fileID, err := r.uploadProject(zipName)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(fileID)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *Runner) runSuites(fileID string) bool {
	suites := make(chan cypress.Suite)
	results := make(chan result, len(r.Project.Suites))
	defer close(results)

	// Create a pool of workers that run the suites.
	log.Info().Int("concurrency", r.Concurrency).Msg("Launching workers.")
	for i := 0; i < r.Concurrency; i++ {
		go r.worker(fileID, suites, results)
	}

	// Submit suites to work on.
	for _, s := range r.Project.Suites {
		suites <- s
	}
	close(suites)

	// Collect results.
	errCount := 0
	completed := 0
	total := len(r.Project.Suites)
	inProgress := total
	passed := true

	progress.Show("Suites completed: %d/%d", completed, total)
	for i := 0; i < total; i++ {
		res := <-results
		// in case one of test suites not passed
		if !res.job.Passed {
			passed = false
		}
		completed++
		inProgress--

		progress.Show("Suites completed: %d/%d", completed, total)
		r.logSuite(res)

		if res.job.ID == "" || res.err != nil {
			errCount++
		}
	}
	progress.Stop()
	logSuitesResult(total, errCount)

	return passed
}

func (r *Runner) worker(fileID string, suites <-chan cypress.Suite, results chan<- result) {
	for s := range suites {
		jobData, err := r.runSuite(s, fileID)

		r := result{
			suiteName: s.Name,
			browser:   s.Browser + " " + s.BrowserVersion,
			job:       jobData,
			err:       err,
		}
		results <- r
	}
}

func (r *Runner) runSuite(s cypress.Suite, fileID string) (job.Job, error) {
	log.Info().Str("suite", s.Name).Str("region", r.Project.Sauce.Region).Msg("Starting job.")

	opts := job.StartOptions{
		User:           credentials.Get().Username,
		AccessKey:      credentials.Get().AccessKey,
		App:            fmt.Sprintf("storage:%s", fileID),
		Suite:          s.Name,
		Framework:      "cypress",
		BrowserName:    s.Browser,
		BrowserVersion: s.BrowserVersion,
		PlatformName:   s.PlatformName,
		Name:           r.Project.Sauce.Metadata.Name + " - " + s.Name,
		Build:          r.Project.Sauce.Metadata.Build,
		Tags:           r.Project.Sauce.Metadata.Tags,
		Tunnel: job.TunnelOptions{
			ID:     r.Project.Sauce.Tunnel.ID,
			Parent: r.Project.Sauce.Tunnel.Parent,
		},
	}

	id, err := r.JobStarter.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{}, err
	}

	log.Info().Str("jobID", id).Msg("Job started.")

	// High interval poll to not oversaturate the job reader with requests.
	j, err := r.JobReader.PollJob(context.Background(), id, 15*time.Second)
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to retrieve job status for suite %s", s.Name)
	}

	if !j.Passed {
		// We may need to differentiate when a job has crashed vs. when there is errors.
		return j, fmt.Errorf("suite '%s' has test failures", s.Name)
	}

	return j, nil
}

func (r *Runner) archiveProject(tempDir string) (string, error) {
	zipName := filepath.Join(tempDir, "app.zip")
	z, err := zip.NewWriter(zipName)
	if err != nil {
		return "", err
	}
	defer z.Close()

	files := []string{
		r.Project.Cypress.ConfigFile,
		r.Project.Cypress.ProjectPath,
	}

	if r.Project.Cypress.EnvFile != "" {
		files = append(files, r.Project.Cypress.EnvFile)
	}

	rcPath := filepath.Join(tempDir, "sauce-runner.json")
	if err := jsonio.WriteFile(rcPath, r.Project); err != nil {
		return "", err
	}
	files = append(files, rcPath)

	for _, f := range files {
		if err := z.Add(f, ""); err != nil {
			return "", err
		}
	}

	return zipName, z.Close()
}

func (r *Runner) uploadProject(filename string) (string, error) {
	progress.Show("Uploading project")
	resp, err := r.ProjectUploader.Upload(filename)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Str("storageId", resp.ID).Msg("Project uploaded.")
	return resp.ID, nil
}

func shouldShowConsole(r *Runner, res result) bool {
	if !res.job.Passed {
		return true
	}
	return r.Project.ShowConsoleLog
}

// logSuite display the result of a suite
func (r *Runner) logSuite(res result) {
	if res.job.ID == "" {
		log.Error().Str("suite", res.suiteName).Msgf("failed to be started")
		log.Error().Str("suite", res.suiteName).Msgf("%s", res.err)
		return
	}
	resultStr := "Passed"
	if !res.job.Passed {
		resultStr = "Failed"
	}
	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), res.job.ID)
	log.Info().Str("suite", res.suiteName).Msgf("Status: %s - %s", resultStr, jobDetailsPage)
	if shouldShowConsole(r, res) {
		r.logSuiteConsole(res)
	}
}

// logSuiteError display the console output when tests from a suite are failing
func (r *Runner) logSuiteConsole(res result) {
	// Display log only when at least it has started
	assetContent, err := r.JobReader.GetJobAssetFileContent(context.Background(), res.job.ID, resto.ConsoleLogAsset)
	if err != nil {
		log.Warn().Str("suite", res.suiteName).Msg("Failed to get job asset.")
	} else {
		log.Info().Msg(fmt.Sprintf("Test %s %s", res.job.ID, resto.ConsoleLogAsset))
		log.Info().Msg(string(assetContent))
	}
}

func logSuitesResult(total, errCount int) {
	log.Info().Msgf("Suites total: %d", total)
	log.Info().Msgf("Suites passed: %d", total-errCount)
	log.Info().Msgf("Suites failed: %d", errCount)
}
