package saucecloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/runtime"

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

	m, err := r.MetadataSearchStrategy.Find(context.Background(), r.MetadataService, testcafe.Kind, r.Project.Testcafe.Version)
	if err != nil {
		r.logFrameworkError(err)
		return exitCode, err
	}
	if err := r.validateFramework(m); err != nil {
		return 1, err
	}
	r.setVersions(m)

	if err := r.setupRuntime(m); err != nil {
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
		return exitCode, err
	}

	if r.Project.DryRun {
		printDryRunSuiteNames(r.getSuiteNames())
		return 0, nil
	}

	passed := r.runSuites(app, otherApps)
	if passed {
		return 0, nil
	}

	return exitCode, nil
}

func (r *TestcafeRunner) setupRuntime(m framework.Metadata) error {
	if !m.SupportGlobalNode() {
		r.Project.NodeVersion = ""
		return nil
	}

	if r.Project.NodeVersion != "" {
		runtimes, err := r.MetadataService.Runtimes(context.Background())
		if err != nil {
			return err
		}
		rt, err := runtime.SelectNode(runtimes, r.Project.NodeVersion)
		if err != nil {
			return err
		}
		if err := rt.Validate(); err != nil {
			return err
		}
		r.Project.NodeVersion = rt.RuntimeVersion
	}

	return nil
}

// setVersions sets the framework and runner versions based on the fetched framework metadata.
// The framework version might be set to `package.json`.
func (r *TestcafeRunner) setVersions(m framework.Metadata) {
	r.Project.Testcafe.Version = m.FrameworkVersion
	r.Project.RunnerVersion = m.CloudRunnerVersion
}

func (r *TestcafeRunner) validateFramework(m framework.Metadata) error {
	if m.IsDeprecated() && !m.IsFlaggedForRemoval() {
		fmt.Print(msg.EOLNotice(testcafe.Kind, r.Project.Testcafe.Version, m.RemovalDate, r.getAvailableVersions(testcafe.Kind)))
	}
	if m.IsFlaggedForRemoval() {
		fmt.Print(msg.RemovalNotice(testcafe.Kind, r.Project.Testcafe.Version, r.getAvailableVersions(testcafe.Kind)))
	}

	for _, s := range r.Project.Suites {
		if s.PlatformName != "" && !framework.HasPlatform(m, s.PlatformName) {
			msg.LogUnsupportedPlatform(s.PlatformName, framework.PlatformNames(m.Platforms))
			return errors.New("unsupported platform")
		}
	}

	return nil
}

func (r *TestcafeRunner) getSuiteNames() []string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return names
}

func (r *TestcafeRunner) runSuites(app string, otherApps []string) bool {
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
			suites = testcafe.SortByHistory(suites, history)
		}
	}

	// Submit suites to work on
	jobsCount := r.calcTestcafeJobsCount(r.Project.Suites)
	go func() {
		for _, s := range suites {
			if len(s.Simulators) > 0 {
				for _, d := range s.Simulators {
					for _, pv := range d.PlatformVersions {
						opts := r.generateStartOpts(s)
						opts.App = app
						opts.OtherApps = otherApps
						opts.PlatformName = d.PlatformName
						opts.DeviceName = d.Name
						opts.PlatformVersion = pv

						jobOpts <- opts
					}
				}
			} else {
				opts := r.generateStartOpts(s)
				opts.App = app
				opts.OtherApps = otherApps
				opts.PlatformName = s.PlatformName

				jobOpts <- opts
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

func (r *TestcafeRunner) generateStartOpts(s testcafe.Suite) job.StartOptions {
	return job.StartOptions{
		ConfigFilePath:   r.Project.ConfigFilePath,
		CLIFlags:         r.Project.CLIFlags,
		DisplayName:      s.Name,
		Timeout:          s.Timeout,
		Suite:            s.Name,
		Framework:        "testcafe",
		FrameworkVersion: r.Project.Testcafe.Version,
		NodeVersion:      r.Project.NodeVersion,
		BrowserName:      s.BrowserName,
		BrowserVersion:   s.BrowserVersion,
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
		TimeZone:         s.TimeZone,
		Visibility:       r.Project.Sauce.Visibility,
		Retries:          r.Project.Sauce.Retries,
		Attempt:          0,
		PassThreshold:    s.PassThreshold,
		SmartRetry: job.SmartRetry{
			FailedOnly: s.SmartRetry.IsRetryFailedOnly(),
		},
	}
}

func (r *TestcafeRunner) calcTestcafeJobsCount(suites []testcafe.Suite) int {
	jobsCount := 0
	for _, s := range suites {
		if len(s.Simulators) > 0 {
			for _, d := range s.Simulators {
				jobsCount += len(d.PlatformVersions)
			}
		} else {
			jobsCount++
		}
	}
	return jobsCount
}
