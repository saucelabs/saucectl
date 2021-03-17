package saucecloud

import (
	"fmt"
	"strings"

	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
)

// CypressRunner represents the Sauce Labs cloud implementation for cypress.
type CypressRunner struct {
	CloudRunner
	Project cypress.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *CypressRunner) RunProject() (int, error) {
	exitCode := 1
	if err := r.checkCypressVersion(); err != nil {
		return exitCode, err
	}

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return 1, err
	}

	var files []string

	if r.Project.RootDir != "" {
		files = append(files, r.Project.RootDir)
	} else {
		files = append(files, r.Project.Cypress.ConfigFile, r.Project.Cypress.ProjectPath)
	}

	if r.Project.Cypress.EnvFile != "" {
		files = append(files, r.Project.Cypress.EnvFile)
	}

	if r.Project.DryRun {
		if err := r.dryRun(r.Project, files, r.Project.Sauce.Sauceignore, r.getSuiteNames()); err != nil {
			return exitCode, err
		}
		return 0, nil
	}
	fileID, err := r.archiveAndUpload(r.Project, files, r.Project.Sauce.Sauceignore)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(fileID)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *CypressRunner) getSuiteNames() string {
	names := []string{}
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}

// checkCypressVersion do several checks before running Cypress tests.
func (r *CypressRunner) checkCypressVersion() error {
	if r.Project.Cypress.Version == "" {
		return fmt.Errorf("missing cypress version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}
	return nil
}

func (r *CypressRunner) runSuites(fileID string) bool {
	jobOpts, results := r.createWorkerPool(r.Project.Sauce.Concurrency)
	defer close(results)

	// Submit suites to work on.
	go func() {
		for _, s := range r.Project.Suites {
			jobOpts <- job.StartOptions{
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
				RunnerVersion:    r.Project.RunnerVersion,
				Experiments:      r.Project.Sauce.Experiments,
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(results, len(r.Project.Suites))
}
