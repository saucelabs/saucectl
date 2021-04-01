package saucecloud

import (
	"fmt"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
)

// EspressoRunner represents the Sauce Labs cloud implementation for cypress.
type EspressoRunner struct {
	CloudRunner
	Project espresso.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *EspressoRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return 1, err
	}

	appFileID, err := r.uploadProject(r.Project.Espresso.App)
	if err != nil {
		return exitCode, err
	}

	testAppFileID, err := r.uploadProject(r.Project.Espresso.TestApp)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(appFileID, testAppFileID)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *EspressoRunner) runSuites(appFileId string, testAppFileId string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results := r.createWorkerPool(r.Project.Sauce.Concurrency)
	defer close(results)

	// Submit suites to work on.
	go func() {
		for _, s := range r.Project.Suites {
			jobOpts <- job.StartOptions{
				App:              fmt.Sprintf("storage:%s", appFileId),
				Suite:            fmt.Sprintf("storage:%s", testAppFileId),
				Framework:        "espresso",
				PlatformName:     s.PlatformName,
				Name:             r.Project.Sauce.Metadata.Name + " - " + s.Name,
				Build:            r.Project.Sauce.Metadata.Build,
				Tags:             r.Project.Sauce.Metadata.Tags,
				Tunnel: job.TunnelOptions{
					ID:     r.Project.Sauce.Tunnel.ID,
					Parent: r.Project.Sauce.Tunnel.Parent,
				},
				Experiments:      r.Project.Sauce.Experiments,
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(results, len(r.Project.Suites))
}
