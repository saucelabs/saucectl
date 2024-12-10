package saucecloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/runtime"
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
func (r *PlaywrightRunner) RunProject(ctx context.Context) (int, error) {
	m, err := r.MetadataSearchStrategy.Find(context.Background(), r.MetadataService, playwright.Kind, r.Project.Playwright.Version)
	if err != nil {
		r.logFrameworkError(err)
		return 1, err
	}
	r.setVersions(m)
	if err := r.validateFramework(m); err != nil {
		return 1, err
	}

	if err := r.setNodeRuntime(m); err != nil {
		return 1, err
	}

	if err := r.validateTunnel(
		r.Project.Sauce.Tunnel.Name,
		r.Project.Sauce.Tunnel.Owner,
		r.Project.DryRun,
		r.Project.Sauce.Tunnel.Timeout,
	); err != nil {
		return 1, err
	}

	app, otherApps, err := r.remoteArchiveProject(ctx, r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore, r.Project.DryRun)
	if err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		printDryRunSuiteNames(r.getSuiteNames())
		return 0, nil
	}

	passed := r.runSuites(ctx, app, otherApps)
	if !passed {
		return 1, nil
	}

	return 0, nil
}

func (r *PlaywrightRunner) setNodeRuntime(m framework.Metadata) error {
	if !m.SupportsRuntime(runtime.NodeRuntime) {
		r.Project.NodeVersion = ""
		return nil
	}

	runtimes, err := r.MetadataService.Runtimes(context.Background())
	if err != nil {
		return err
	}
	// Set the default version if the runner supports global Node.js
	// but no version is specified by user.
	if r.Project.NodeVersion == "" {
		d, err := runtime.GetDefault(runtimes, runtime.NodeRuntime)
		if err != nil {
			return err
		}
		r.Project.NodeVersion = d.Version
		return nil
	}

	rt, err := runtime.Find(runtimes, runtime.NodeRuntime, r.Project.NodeVersion)
	if err != nil {
		return err
	}
	if err := rt.Validate(runtimes); err != nil {
		return err
	}
	r.Project.NodeVersion = rt.Version

	return nil
}

// setVersions sets the framework and runner versions based on the fetched framework metadata.
// The framework version might be set to `package.json`.
func (r *PlaywrightRunner) setVersions(m framework.Metadata) {
	r.Project.Playwright.Version = m.FrameworkVersion
	// RunnerVersion can be set via `--runner-version`.
	// If not provided, it uses the fetched framework runner version.
	if r.Project.RunnerVersion == "" {
		r.Project.RunnerVersion = m.CloudRunnerVersion
	}
}

func (r *PlaywrightRunner) validateFramework(m framework.Metadata) error {
	if m.IsDeprecated() && !m.IsFlaggedForRemoval() {
		fmt.Print(msg.EOLNotice(playwright.Kind, r.Project.Playwright.Version, m.RemovalDate, r.getAvailableVersions(playwright.Kind)))
	}
	if m.IsFlaggedForRemoval() {
		fmt.Print(msg.RemovalNotice(playwright.Kind, r.Project.Playwright.Version, r.getAvailableVersions(playwright.Kind)))
	}

	for i, s := range r.Project.Suites {
		if s.PlatformName != "" && !framework.HasPlatform(m, s.PlatformName) {
			msg.LogUnsupportedPlatform(s.PlatformName, framework.PlatformNames(m.Platforms))
			return errors.New("unsupported platform")
		}
		r.Project.Suites[i].Params.BrowserVersion = m.BrowserDefaults[PlaywrightBrowserMap[s.Params.BrowserName]]
	}
	return nil
}

func (r *PlaywrightRunner) getSuiteNames() []string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return names
}

func (r *PlaywrightRunner) runSuites(ctx context.Context, app string, otherApps []string) bool {
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
				NodeVersion:      r.Project.NodeVersion,
				BrowserName:      s.Params.BrowserName,
				BrowserVersion:   s.Params.BrowserVersion,
				PlatformName:     s.PlatformName,
				Name:             s.Name,
				Build:            r.Project.Sauce.Metadata.Build,
				Tags:             r.Project.Sauce.Metadata.Tags,
				Tunnel: job.TunnelOptions{
					Name:  r.Project.Sauce.Tunnel.Name,
					Owner: r.Project.Sauce.Tunnel.Owner,
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

	return r.collectResults(results, len(r.Project.Suites))
}
