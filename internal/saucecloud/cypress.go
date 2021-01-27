package saucecloud

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/cli/dots"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
)

// CypressRunner represents the Sauce Labs cloud implementation for cypress.
type CypressRunner struct {
	CloudRunner
	Project         cypress.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *CypressRunner) RunProject() (int, error) {
	exitCode := 1
	if err := r.checkCypressVersion(); err != nil {
		return exitCode, err
	}

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

	files := []string{
		r.Project.Cypress.ConfigFile,
		r.Project.Cypress.ProjectPath,
	}

	if r.Project.Cypress.EnvFile != "" {
		files = append(files, r.Project.Cypress.EnvFile)
	}

	zipName, err := r.archiveProject(r.Project, tempDir, files)
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

// checkCypressVersion do several checks before running Cypress tests.
func (r *CypressRunner) checkCypressVersion() error {
	if r.Project.Cypress.Version == "" {
		return fmt.Errorf("no cypress version provided")
	}
	return nil
}

func (r *CypressRunner) runSuites(fileID string) bool {
	suites := make(chan cypress.Suite)
	results := make(chan result, len(r.Project.Suites))
	defer close(results)

	// Create a pool of workers that run the suites.
	r.Project.Sauce.Concurrency = concurrency.Min(r.CCYReader, r.Project.Sauce.Concurrency)
	log.Info().Int("concurrency", r.Project.Sauce.Concurrency).Msg("Launching workers.")
	for i := 0; i < r.Project.Sauce.Concurrency; i++ {
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

	waiter := dots.New(1)
	waiter.Start()
	for i := 0; i < total; i++ {
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
		log.Info().Msg(fmt.Sprintf("Suites completed: %d/%d", completed, total))
		r.logSuite(res)

		if res.job.ID == "" || res.err != nil {
			errCount++
		}
	}
	waiter.Stop()

	log.Info().Msgf("Suites total: %d", total)
	log.Info().Msgf("Suites passed: %d", total-errCount)
	log.Info().Msgf("Suites failed: %d", errCount)

	return passed
}

func (r *CypressRunner) worker(fileID string, suites <-chan cypress.Suite, results chan<- result) {
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

func (r *CypressRunner) runSuite(s cypress.Suite, fileID string) (job.Job, error) {
	log.Info().Str("suite", s.Name).Str("region", r.Project.Sauce.Region).Msg("Starting job.")

	opts := job.StartOptions{
		User:             credentials.Get().Username,
		AccessKey:        credentials.Get().AccessKey,
		App:              fmt.Sprintf("storage:%s", fileID),
		Suite:            s.Name,
		Framework:        "cypress",
		FrameworkVersion: r.Project.Cypress.Version,
		BrowserName:      s.Browser,
		BrowserVersion:   s.BrowserVersion,
		PlatformName:     s.PlatformName,
		Name:             r.Project.Sauce.Metadata.Name + " - " + s.Name,
		Build:            r.Project.Sauce.Metadata.Build,
		Tags:             r.Project.Sauce.Metadata.Tags,
		Tunnel: job.TunnelOptions{
			ID:     r.Project.Sauce.Tunnel.ID,
			Parent: r.Project.Sauce.Tunnel.Parent,
		},
		ScreenResolution: s.ScreenResolution,
	}

	id, err := r.JobStarter.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{}, err
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), id)
	log.Info().Msg(fmt.Sprintf("Job started - %s", jobDetailsPage))

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
