package saucecloud

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/puppeteer/replay"
)

// ReplayRunner represents the Sauce Labs cloud implementation for puppeteer-replay.
type ReplayRunner struct {
	CloudRunner
	Project replay.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *ReplayRunner) RunProject() (int, error) {
	exitCode := 1

	m, err := r.MetadataSearchStrategy.Find(context.Background(), r.MetadataService, replay.Kind, "latest")
	if err != nil {
		r.logFrameworkError(err)
		return exitCode, err
	}
	if r.Project.RunnerVersion == "" {
		r.Project.RunnerVersion = m.CloudRunnerVersion
	}

	for _, s := range r.Project.Suites {
		if s.Platform != "" && !framework.HasPlatform(m, s.Platform) {
			msg.LogUnsupportedPlatform(s.Platform, framework.PlatformNames(m.Platforms))
			return 1, errors.New("unsupported platform")
		}
	}

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner); err != nil {
		return 1, err
	}

	var files []string
	var suiteNames []string
	for _, suite := range r.Project.Suites {
		suiteNames = append(suiteNames, suite.Name)
		files = append(files, suite.Recording)
	}

	fileURI, err := r.remoteArchiveFiles(r.Project, files, "", r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	if r.Project.DryRun {
		log.Info().Msgf("The following test suites would have run: %s.", suiteNames)
		return 0, nil
	}

	passed := r.runSuites(fileURI)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
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
				RunnerVersion:    r.Project.RunnerVersion,
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
