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
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/storage"
)

// SauceLabsURL represents sauce labs app URL.
var SauceLabsURL = "https://app.saucelabs.com"

// Runner represents the Sauce Labs cloud implementation for cypress.
type Runner struct {
	Project         cypress.Project
	ProjectUploader storage.ProjectUploader
	JobStarter      job.Starter
	JobReader       job.Reader
	Concurrency     int
}

type result struct {
	suiteName string
	browser   string
	job       job.Job
	err       error
}

// RunProject runs the tests defined in cypress.Project.
func (r *Runner) RunProject() (int, error) {
	log.Error().Msg("Caution: Not yet implemented.") // TODO remove debug

	// Archive the project files.
	tempDir, err := ioutil.TempDir(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return 1, err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(tempDir)
	if err != nil {
		return 1, err
	}

	fileID, err := r.uploadProject(zipName)
	if err != nil {
		return 1, err
	}

	errCount := r.runSuites(fileID)

	// FIXME forcing an error, since this feature is not fully implemented yet
	errCount = 1
	return errCount, nil
}

func (r *Runner) runSuites(fileID string) int {
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
	inprogress := total

	log.Info().Msgf("Suites completed: %d in progress: %d", completed, inprogress)
	for i := 0; i < total; i++ {
		res := <-results
		completed++
		inprogress--
		logSuite(completed, inprogress, res.suiteName, res.browser, res.job.Passed)

		if res.err != nil {
			assetContent, err := r.JobReader.GetJobAssetFileContent(context.Background(), res.job.ID, resto.ConsoleLogAsset)
			if err != nil {
				log.Warn().Str("suite", res.suiteName).Msg("Failed to get job asset.")
			} else {
				fmt.Println(string(assetContent))
			}
			errCount++
		}
		log.Info().Msgf("Open job details page: %s", SauceLabsURL+"/tests/"+res.job.ID)
	}

	logSuitesResult(total, errCount)

	return errCount
}

func (r *Runner) worker(fileID string, suites <-chan cypress.Suite, results chan<- result) {
	for s := range suites {
		job, err := r.runSuite(s, fileID)
		r := result{
			suiteName: s.Name,
			browser:   s.Browser + " " + s.BrowserVersion,
			job:       job,
			err:       err,
		}
		results <- r
	}
}

func (r *Runner) runSuite(s cypress.Suite, fileID string) (job.Job, error) {
	log.Info().Str("suite", s.Name).Msg("Starting job.")

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
		// TODO do we need to differentiate test passes/failure vs. job failure (failed to start, crashed)?
		return job.Job{}, fmt.Errorf("suite '%s' has test failures", s.Name)
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
	log.Info().Str("fileID", resp.ID).Msg("Project uploaded.")
	return resp.ID, nil
}

func logSuite(completed, inprogress int, suitName, browser string, passed bool) {
	log.Info().Msgf("Suites completed: %d in progress: %d", completed, inprogress)
	log.Info().Msgf("Suite name: %s", suitName)
	log.Info().Msgf("Browser: %s", browser)
	log.Info().Msgf("Passed: %t", passed)
}

func logSuitesResult(total, errCount int) {
	log.Info().Msgf("Suits total: %d", total)
	log.Info().Msgf("Suits passed: %d", total-errCount)
	log.Info().Msgf("Suits failed: %d", errCount)
}
