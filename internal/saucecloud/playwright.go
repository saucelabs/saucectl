package saucecloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/playwright"
)

// PlaywrightRunner represents the Sauce Labs cloud implementation for playwright.
type PlaywrightRunner struct {
	CloudRunner
	Project playwright.Project
}

var PlaywrightBrowserMap = map[string]string{
	"chromium": "playwright-chromium",
	"firefox":  "playwright-firefox",
	"webkit":   "playwright-webkit",
}

// RunProject runs the tests defined in cypress.Project.
func (r *PlaywrightRunner) RunProject() (int, error) {
	var deprecationMessage string
	exitCode := 1

	m, err := r.MetadataSearchStrategy.Find(context.Background(), r.MetadataService, playwright.Kind, r.Project.Playwright.Version)
	if err != nil {
		r.logFrameworkError(err)
		return exitCode, err
	}
	r.Project.Playwright.Version = m.FrameworkVersion
	if r.Project.RunnerVersion == "" {
		r.Project.RunnerVersion = m.CloudRunnerVersion
	}

	if m.IsDeprecated() && !m.IsFlaggedForRemoval() {
		deprecationMessage = r.deprecationMessage(playwright.Kind, r.Project.Playwright.Version, m.RemovalDate)
		fmt.Print(deprecationMessage)
	}
	if m.IsFlaggedForRemoval() {
		deprecationMessage = r.flaggedForRemovalMessage(playwright.Kind, r.Project.Playwright.Version)
		fmt.Print(deprecationMessage)
	}

	for i, s := range r.Project.Suites {
		if s.PlatformName != "" && !framework.HasPlatform(m, s.PlatformName) {
			msg.LogUnsupportedPlatform(s.PlatformName, framework.PlatformNames(m.Platforms))
			return 1, errors.New("unsupported platform")
		}
		r.Project.Suites[i].Params.BrowserVersion = m.BrowserDefaults[PlaywrightBrowserMap[s.Params.BrowserName]]
	}

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner, r.Project.DryRun); err != nil {
		return 1, err
	}

	app, otherApps, err := r.remoteArchiveProject(r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore, r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	if r.Project.DryRun {
		printDryRunSuiteNames(r.getSuiteNames())
		return 0, nil
	}

	passed := r.runSuites(app, otherApps)
	if passed {
		exitCode = 0
	}

	if deprecationMessage != "" {
		fmt.Print(deprecationMessage)
	}

	return exitCode, nil
}

func (r *PlaywrightRunner) getSuiteNames() []string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return names
}

func (r *PlaywrightRunner) runSuites(app string, otherApps []string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency, r.Project.Sauce.Retries)
	if err != nil {
		return false
	}
	defer close(results)

	suites := r.Project.Suites
	if r.Project.Sauce.LaunchOrder != "" {
		history, err := r.getHistory(r.Project.Sauce.LaunchOrder)
		if err != nil {
			log.Warn().Err(err).Msg(msg.RetrieveJobHistoryError)
		} else {
			suites = playwright.SortByHistory(suites, history)
		}
	}
	// Submit suites to work on.
	go func() {
		for _, s := range suites {
			// Define frameworkVersion if not set at suite level
			if s.PlaywrightVersion == "" {
				s.PlaywrightVersion = r.Project.Playwright.Version
			}
			jobOpts <- job.StartOptions{
				ConfigFilePath:   r.Project.ConfigFilePath,
				CLIFlags:         r.Project.CLIFlags,
				DisplayName:      s.Name,
				Timeout:          s.Timeout,
				App:              app,
				OtherApps:        otherApps,
				Suite:            s.Name,
				Framework:        "playwright",
				FrameworkVersion: s.PlaywrightVersion,
				BrowserName:      s.Params.BrowserName,
				BrowserVersion:   s.Params.BrowserVersion,
				PlatformName:     s.PlatformName,
				Name:             s.Name,
				Build:            r.Project.Sauce.Metadata.Build,
				Tags:             r.Project.Sauce.Metadata.Tags,
				Tunnel: job.TunnelOptions{
					ID:     r.Project.Sauce.Tunnel.Name,
					Parent: r.Project.Sauce.Tunnel.Owner,
				},
				ScreenResolution: s.ScreenResolution,
				RunnerVersion:    r.Project.RunnerVersion,
				Experiments:      r.Project.Sauce.Experiments,
				Attempt:          0,
				Retries:          r.Project.Sauce.Retries,
				TimeZone:         s.TimeZone,
				Visibility:       r.Project.Sauce.Visibility,
				PassThreshold:    s.PassThreshold,
				SmartRetry: job.SmartRetry{
					FailedOnly: s.SmartRetry.IsRetryFailedOnly(),
				},
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, len(r.Project.Suites))
}
