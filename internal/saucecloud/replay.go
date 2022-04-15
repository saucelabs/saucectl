package saucecloud

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/puppeteer/replay"
	"strings"

	"github.com/saucelabs/saucectl/internal/job"
)

// ReplayRunner represents the Sauce Labs cloud implementation for puppeteer-replay.
type ReplayRunner struct {
	CloudRunner
	Project replay.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *ReplayRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner); err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		// TODO implement dry run
		return 0, errors.New("dry run not implemented")
	}

	var files []string
	for _, suite := range r.Project.Suites {
		files = append(files, suite.Recording)
	}

	fileURI, err := r.remoteArchiveFiles(r.Project, files, "")
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(fileURI)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *ReplayRunner) getSuiteNames() string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}

	return strings.Join(names, ", ")
}

func (r *ReplayRunner) runSuites(fileURI string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency, r.Project.Sauce.Retries)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on.
	go func() {
		for _, s := range r.Project.Suites {
			jobOpts <- job.StartOptions{
				ConfigFilePath:   r.Project.ConfigFilePath,
				DisplayName:      s.Name,
				Timeout:          s.Timeout,
				App:              fileURI,
				Suite:            s.Name,
				Framework:        "puppeteer-replay",
				FrameworkVersion: "latest",
				BrowserName:      s.BrowserName,
				BrowserVersion:   s.BrowserVersion,
				PlatformName:     s.Platform,
				Name:             s.Name,
				Build:            r.Project.Sauce.Metadata.Build,
				Tags:             r.Project.Sauce.Metadata.Tags,
				Tunnel: job.TunnelOptions{
					ID:     r.Project.Sauce.Tunnel.Name,
					Parent: r.Project.Sauce.Tunnel.Owner,
				},
				Experiments: r.Project.Sauce.Experiments,
				Attempt:     0,
				Retries:     r.Project.Sauce.Retries,
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, len(r.Project.Suites))
}
