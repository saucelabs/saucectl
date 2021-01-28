package saucecloud

import (
	"context"
	"fmt"
	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
	"io/ioutil"
	"os"
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
		return fmt.Errorf("Missing cypress version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers")
	}
	return nil
}

func (r *CypressRunner) runSuites(fileID string) bool {
	jobOpts, results := r.createWorkerPool(r.Project.Sauce.Concurrency)
	defer close(results)

	// Submit suites to work on.
	for _, s := range r.Project.Suites {
		jobOpts <- job.StartOptions{
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
	}
	close(jobOpts)

	return r.collectResults(results, len(r.Project.Suites))
}
