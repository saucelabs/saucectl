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
func (r *TestcafeRunner) RunProject(ctx context.Context) (int, error) {
	m, err := r.MetadataSearchStrategy.Find(ctx, r.MetadataService, testcafe.Kind, r.Project.Testcafe.Version)
	if err != nil {
		r.logFrameworkError(ctx, err)
		return 1, err
	}
	r.setVersions(m)
	if err := r.validateFramework(ctx, m); err != nil {
		return 1, err
	}

	if err := r.setNodeRuntime(ctx, m); err != nil {
		return 1, err
	}

	if err := r.validateTunnel(
		ctx,
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

func (r *TestcafeRunner) setNodeRuntime(ctx context.Context, m framework.Metadata) error {
	if !m.SupportsRuntime(runtime.NodeRuntime) {
		r.Project.NodeVersion = ""
		return nil
	}

	runtimes, err := r.MetadataService.Runtimes(ctx)
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
func (r *TestcafeRunner) setVersions(m framework.Metadata) {
	r.Project.Testcafe.Version = m.FrameworkVersion
	// RunnerVersion can be set via `--runner-version`.
	// If not provided, it uses the fetched framework runner version.
	if r.Project.RunnerVersion == "" {
		r.Project.RunnerVersion = m.CloudRunnerVersion
	}
}

func (r *TestcafeRunner) validateFramework(ctx context.Context, m framework.Metadata) error {
	if m.IsDeprecated() && !m.IsFlaggedForRemoval() {
		fmt.Print(msg.EOLNotice(testcafe.Kind, r.Project.Testcafe.Version, m.RemovalDate, r.getAvailableVersions(ctx, testcafe.Kind)))
	}
	if m.IsFlaggedForRemoval() {
		fmt.Print(msg.RemovalNotice(testcafe.Kind, r.Project.Testcafe.Version, r.getAvailableVersions(ctx, testcafe.Kind)))
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

func (r *TestcafeRunner) runSuites(ctx context.Context, app string, otherApps []string) bool {
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

	return r.collectResults(results, jobsCount)
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
			Name:  r.Project.Sauce.Tunnel.Name,
			Owner: r.Project.Sauce.Tunnel.Owner,
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
