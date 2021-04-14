package saucecloud

import (
	"fmt"
	"strings"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/testcafe"
)

// TestcafeRunner represents the SauceLabs cloud implementation
type TestcafeRunner struct {
	CloudRunner
	Project testcafe.Project
}

// RunProject runs the defined tests on sauce cloud
func (r *TestcafeRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		if err := r.dryRun(r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore, r.getSuiteNames()); err != nil {
			return exitCode, err
		}
		return 0, nil
	}

	fileID, err := r.archiveAndUpload(r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore)
	if err != nil {
		return exitCode, err
	}
	passed := r.runSuites(fileID)
	if passed {
		return 0, nil
	}

	return exitCode, nil
}

func (r *TestcafeRunner) getSuiteNames() string {
	names := []string{}
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}

	return strings.Join(names, ", ")
}

func (r *TestcafeRunner) runSuites(fileID string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on
	for _, s := range r.Project.Suites {
		jobOpts <- job.StartOptions{
			App:              fmt.Sprintf("storage:%s", fileID),
			Suite:            s.Name,
			Framework:        "testcafe",
			FrameworkVersion: r.Project.Testcafe.Version,
			BrowserName:      s.BrowserName,
			BrowserVersion:   s.BrowserVersion,
			PlatformName:     s.PlatformName,
			Name:             fmt.Sprintf("%s - %s", r.Project.Sauce.Metadata.Name, s.Name),
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

	return r.collectResults(r.Project.Artifacts.Download, results, len(r.Project.Suites))
}
