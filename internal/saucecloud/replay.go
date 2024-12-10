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
func (r *ReplayRunner) RunProject(ctx context.Context) (int, error) {
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

	if err := r.validateTunnel(
		r.Project.Sauce.Tunnel.Name,
		r.Project.Sauce.Tunnel.Owner,
		r.Project.DryRun,
		r.Project.Sauce.Tunnel.Timeout,
	); err != nil {
		return 1, err
	}

	var files []string
	var suiteNames []string
	for _, suite := range r.Project.Suites {
		suiteNames = append(suiteNames, suite.Name)
		files = append(files, suite.Recording)
	}

	fileURIs, err := r.remoteArchiveFiles(ctx, r.Project, files, "", r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	if r.Project.DryRun {
		printDryRunSuiteNames(suiteNames)
		return 0, nil
	}

	passed := r.runSuites(ctx, fileURIs)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *ReplayRunner) runSuites(ctx context.Context, fileURI string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results := r.createWorkerPool(ctx, r.Project.Sauce.Concurrency, r.Project.Sauce.Retries)
	defer close(results)

	suites := r.Project.Suites
	if r.Project.Sauce.LaunchOrder != "" {
		history, err := r.getHistory(r.Project.Sauce.LaunchOrder)
		if err != nil {
			log.Warn().Err(err).Msg(msg.RetrieveJobHistoryError)
		} else {
			suites = replay.SortByHistory(suites, history)
		}
	}
	// Submit suites to work on.
	go func() {
		for _, s := range suites {
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
					Name:  r.Project.Sauce.Tunnel.Name,
					Owner: r.Project.Sauce.Tunnel.Owner,
				},
				Experiments:   r.Project.Sauce.Experiments,
				Attempt:       0,
				Retries:       r.Project.Sauce.Retries,
				Visibility:    r.Project.Sauce.Visibility,
				PassThreshold: s.PassThreshold,
			}
		}
	}()

	return r.collectResults(results, len(r.Project.Suites))
}
