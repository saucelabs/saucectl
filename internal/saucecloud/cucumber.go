package saucecloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cucumber"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/runtime"

	"github.com/saucelabs/saucectl/internal/job"
)

// CucumberRunner represents the SauceLabs cloud implementation
type CucumberRunner struct {
	CloudRunner
	Project cucumber.Project
}

// RunProject runs the defined tests on sauce cloud
func (r *CucumberRunner) RunProject() (int, error) {
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

	app, otherApps, err := r.remoteArchiveProject(r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore, r.Project.DryRun)
	if err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		printDryRunSuiteNames(r.getSuiteNames())
		return 0, nil
	}

	passed := r.runSuites(app, otherApps)
	if !passed {
		return 1, nil
	}

	return 0, nil
}

// setVersions sets the framework and runner versions based on the fetched framework metadata.
// The framework version might be set to `package.json`.
func (r *CucumberRunner) setVersions(m framework.Metadata) {
	r.Project.Playwright.Version = m.FrameworkVersion
	// RunnerVersion can be set via `--runner-version`.
	// If not provided, it uses the fetched framework runner version.
	if r.Project.RunnerVersion == "" {
		r.Project.RunnerVersion = m.CloudRunnerVersion
	}
}

func (r *CucumberRunner) validateFramework(m framework.Metadata) error {
	if m.IsDeprecated() && !m.IsFlaggedForRemoval() {
		fmt.Print(msg.EOLNotice(playwright.Kind, r.Project.Playwright.Version, m.RemovalDate, r.getAvailableVersions(playwright.Kind)))
	}
	if m.IsFlaggedForRemoval() {
		fmt.Print(msg.RemovalNotice(playwright.Kind, r.Project.Playwright.Version, r.getAvailableVersions(playwright.Kind)))
	}

	for _, s := range r.Project.Suites {
		if s.PlatformName != "" && !framework.HasPlatform(m, s.PlatformName) {
			msg.LogUnsupportedPlatform(s.PlatformName, framework.PlatformNames(m.Platforms))
			return errors.New("unsupported platform")
		}
	}
	return nil
}

func (r *CucumberRunner) setNodeRuntime(metadata framework.Metadata) error {
	if !metadata.SupportsRuntime(runtime.NodeRuntime) {
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

func (r *CucumberRunner) getSuiteNames() []string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return names
}

func (r *CucumberRunner) runSuites(app string, otherApps []string) bool {
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
			suites = cucumber.SortByHistory(suites, history)
		}
	}

	// Submit suites to work on
	go func() {
		for _, s := range suites {
			jobOpts <- job.StartOptions{
				ConfigFilePath:   r.Project.ConfigFilePath,
				DisplayName:      s.Name,
				App:              app,
				OtherApps:        otherApps,
				Suite:            s.Name,
				Framework:        "playwright",
				FrameworkVersion: r.Project.Playwright.Version,
				NodeVersion:      r.Project.NodeVersion,
				BrowserName:      s.BrowserName,
				BrowserVersion:   s.BrowserVersion,
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
