package saucecloud

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"

	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/cli/dots"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/playwright"
)

// PlaywrightRunner represents the Sauce Labs cloud implementation for cypress.
type PlaywrightRunner struct {
	CloudRunner
	Project playwright.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *PlaywrightRunner) RunProject() (int, error) {
	exitCode := 1

	// Archive the project files.
	tempDir, err := ioutil.TempDir(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return exitCode, err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(r.Project, tempDir, []string{r.Project.Playwright.LocalProjectPath})
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

func (r *PlaywrightRunner) runSuites(fileID string) bool {
	suites := make(chan playwright.Suite)
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
		// Define frameworkVersion if not set at suite level
		if s.PlaywrightVersion == "" {
			s.PlaywrightVersion = r.Project.Playwright.Version
		}
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

func (r *PlaywrightRunner) worker(fileID string, suites <-chan playwright.Suite, results chan<- result) {
	for s := range suites {
		opts := job.StartOptions{
			User:             credentials.Get().Username,
			AccessKey:        credentials.Get().AccessKey,
			App:              fmt.Sprintf("storage:%s", fileID),
			Suite:            s.Name,
			Framework:        "playwright",
			FrameworkVersion: s.PlaywrightVersion,
			BrowserName:      s.Params.BrowserName,
			BrowserVersion:   s.PlaywrightVersion,
			PlatformName:     s.PlatformName,
			Name:             r.Project.Sauce.Metadata.Name + " - " + s.Name,
			Build:            r.Project.Sauce.Metadata.Build,
			Tags:             r.Project.Sauce.Metadata.Tags,
			Tunnel: job.TunnelOptions{
				ID:     r.Project.Sauce.Tunnel.ID,
				Parent: r.Project.Sauce.Tunnel.Parent,
			},
		}

		jobData, err := r.runJob(opts)

		r := result{
			suiteName: s.Name,
			browser:   s.Params.BrowserName,
			job:       jobData,
			err:       err,
		}
		results <- r
	}
}
