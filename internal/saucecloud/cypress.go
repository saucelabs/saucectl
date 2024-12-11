package saucecloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/cypress/suite"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/runtime"
)

// CypressRunner represents the Sauce Labs cloud implementation for cypress.
type CypressRunner struct {
	CloudRunner
	Project cypress.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *CypressRunner) RunProject(ctx context.Context) (int, error) {
	m, err := r.MetadataSearchStrategy.Find(context.Background(), r.MetadataService, cypress.Kind, r.Project.GetVersion())
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
		ctx,
		r.Project.GetSauceCfg().Tunnel.Name,
		r.Project.GetSauceCfg().Tunnel.Owner,
		r.Project.IsDryRun(),
		r.Project.GetSauceCfg().Tunnel.Timeout,
	); err != nil {
		return 1, err
	}

	app, otherApps, err := r.remoteArchiveProject(ctx, r.Project, r.Project.GetRootDir(), r.Project.GetSauceCfg().Sauceignore, r.Project.IsDryRun())
	if err != nil {
		return 1, err
	}

	if r.Project.IsDryRun() {
		log.Info().Msgf("The following test suites would have run: [%s].", r.Project.GetSuiteNames())
		return 0, nil
	}

	passed := r.runSuites(ctx, app, otherApps)
	if !passed {
		return 1, nil
	}

	return 0, nil
}

func (r *CypressRunner) setNodeRuntime(m framework.Metadata) error {
	if !m.SupportsRuntime(runtime.NodeRuntime) {
		r.Project.SetNodeVersion("")
		return nil
	}

	runtimes, err := r.MetadataService.Runtimes(context.Background())
	if err != nil {
		return err
	}
	// Set the default version if the runner supports global Node.js
	// but no version is specified by user.
	if r.Project.GetNodeVersion() == "" {
		d, err := runtime.GetDefault(runtimes, runtime.NodeRuntime)
		if err != nil {
			return err
		}
		r.Project.SetNodeVersion(d.Version)
		return nil
	}

	rt, err := runtime.Find(runtimes, runtime.NodeRuntime, r.Project.GetNodeVersion())
	if err != nil {
		return err
	}
	if err := rt.Validate(runtimes); err != nil {
		return err
	}
	r.Project.SetNodeVersion(rt.Version)

	return nil
}

// setVersions sets the framework and runner versions based on the fetched framework metadata.
// The framework version might be set to `package.json`.
func (r *CypressRunner) setVersions(m framework.Metadata) {
	r.Project.SetVersion(m.FrameworkVersion)
	// RunnerVersion can be set via `--runner-version`.
	// If not provided, it uses the fetched framework runner version.
	if r.Project.GetRunnerVersion() == "" {
		r.Project.SetRunnerVersion(m.CloudRunnerVersion)
	}
}

func (r *CypressRunner) validateFramework(m framework.Metadata) error {
	cyVersion := r.Project.GetVersion()
	if m.IsDeprecated() && !m.IsFlaggedForRemoval() {
		fmt.Print(msg.EOLNotice(cypress.Kind, cyVersion, m.RemovalDate, r.getAvailableVersions(cypress.Kind)))
	}
	if m.IsFlaggedForRemoval() {
		fmt.Print(msg.RemovalNotice(cypress.Kind, cyVersion, r.getAvailableVersions(cypress.Kind)))
	}

	for _, s := range r.Project.GetSuites() {
		if s.PlatformName != "" && !framework.HasPlatform(m, s.PlatformName) {
			msg.LogUnsupportedPlatform(s.PlatformName, framework.PlatformNames(m.Platforms))
			return errors.New("unsupported platform")
		}
	}
	return nil
}

func (r *CypressRunner) runSuites(ctx context.Context, app string, otherApps []string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)
	jobOpts, results := r.createWorkerPool(ctx, r.Project.GetSauceCfg().Concurrency, r.Project.GetSauceCfg().Retries)
	defer close(results)

	suites := r.Project.GetSuites()
	if r.Project.GetSauceCfg().LaunchOrder != "" {
		history, err := r.getHistory(r.Project.GetSauceCfg().LaunchOrder)
		if err != nil {
			log.Warn().Err(err).Msg(msg.RetrieveJobHistoryError)
		} else {
			suites = suite.SortByHistory(suites, history)
		}
	}

	// Submit suites to work on.
	go func() {
		for _, s := range suites {
			smartRetry := r.Project.GetSmartRetry(s.Name)
			jobOpts <- job.StartOptions{
				ConfigFilePath:   r.Project.GetCfgPath(),
				CLIFlags:         r.Project.GetCLIFlags(),
				DisplayName:      s.Name,
				Timeout:          s.Timeout,
				App:              app,
				OtherApps:        otherApps,
				Suite:            s.Name,
				Framework:        "cypress",
				FrameworkVersion: r.Project.GetVersion(),
				NodeVersion:      r.Project.GetNodeVersion(),
				BrowserName:      s.Browser,
				BrowserVersion:   s.BrowserVersion,
				PlatformName:     s.PlatformName,
				Name:             s.Name,
				Build:            r.Project.GetSauceCfg().Metadata.Build,
				Tags:             r.Project.GetSauceCfg().Metadata.Tags,
				Tunnel: job.TunnelOptions{
					Name:  r.Project.GetSauceCfg().Tunnel.Name,
					Owner: r.Project.GetSauceCfg().Tunnel.Owner,
				},
				ScreenResolution: s.ScreenResolution,
				RunnerVersion:    r.Project.GetRunnerVersion(),
				Experiments:      r.Project.GetSauceCfg().Experiments,
				Attempt:          0,
				Retries:          r.Project.GetSauceCfg().Retries,
				TimeZone:         s.TimeZone,
				Visibility:       r.Project.GetSauceCfg().Visibility,
				PassThreshold:    s.PassThreshold,
				SmartRetry: job.SmartRetry{
					FailedOnly: smartRetry.IsRetryFailedOnly(),
				},
			}
		}
	}()

	return r.collectResults(results, r.Project.GetSuiteCount())
}
