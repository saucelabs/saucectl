package saucecloud

import (
	"fmt"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/playwright"
)

// PlaywrightRunner represents the Sauce Labs cloud implementation for playwright.
type PlaywrightRunner struct {
	CloudRunner
	Project playwright.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *PlaywrightRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return 1, err
	}

	fileID, err := r.archiveAndUpload(r.Project, []string{r.Project.Playwright.LocalProjectPath}, r.Project.Sauce.Sauceignore)
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
	jobOpts, results := r.createWorkerPool(r.Project.Sauce.Concurrency)
	defer close(results)

	// Submit suites to work on.
	go func() {
		for _, s := range r.Project.Suites {
			// Define frameworkVersion if not set at suite level
			if s.PlaywrightVersion == "" {
				s.PlaywrightVersion = r.Project.Playwright.Version
			}
			jobOpts <- job.StartOptions{
				App:              fmt.Sprintf("storage:%s", fileID),
				Suite:            s.Name,
				Framework:        "playwright",
				FrameworkVersion: s.PlaywrightVersion,
				BrowserName:      s.Params.BrowserName,
				BrowserVersion:   "",
				PlatformName:     s.PlatformName,
				Name:             r.Project.Sauce.Metadata.Name + " - " + s.Name,
				Build:            r.Project.Sauce.Metadata.Build,
				Tags:             r.Project.Sauce.Metadata.Tags,
				Tunnel: job.TunnelOptions{
					ID:     r.Project.Sauce.Tunnel.ID,
					Parent: r.Project.Sauce.Tunnel.Parent,
				},
				ScreenResolution: s.ScreenResolution,
				RunnerVersion:    r.Project.RunnerVersion,
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(results, len(r.Project.Suites))
}
