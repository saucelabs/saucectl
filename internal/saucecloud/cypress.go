package saucecloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
)

// CypressRunner represents the Sauce Labs cloud implementation for cypress.
type CypressRunner struct {
	CloudRunner
	Project cypress.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *CypressRunner) RunProject() (int, error) {
	var deprecationMessage string
	exitCode := 1

	cyVersion := r.Project.GetVersion()
	m, err := r.MetadataSearchStrategy.Find(context.Background(), r.MetadataService, cypress.Kind, cyVersion)
	if err != nil {
		r.logFrameworkError(err)
		return exitCode, err
	}
	r.Project.SetVersion(m.FrameworkVersion)
	if r.Project.GetRunnerVersion() == "" {
		r.Project.SetRunnerVersion(m.CloudRunnerVersion)
	}

	if m.Deprecated {
		deprecationMessage = r.deprecationMessage(cypress.Kind, cyVersion)
		fmt.Print(deprecationMessage)
	}

	for _, s := range r.Project.GetSuites() {
		if s.PlatformName != "" && !framework.HasPlatform(m, s.PlatformName) {
			msg.LogUnsupportedPlatform(s.PlatformName, framework.PlatformNames(m.Platforms))
			return 1, errors.New("unsupported platform")
		}
	}

	if err := r.validateTunnel(r.Project.GetSauceCfg().Tunnel.Name, r.Project.GetSauceCfg().Tunnel.Owner); err != nil {
		return 1, err
	}

	if r.Project.IsDryRun() {
		if err := r.dryRun(r.Project, r.Project.GetRootDir(), r.Project.GetSauceCfg().Sauceignore, r.Project.GetSuiteNames()); err != nil {
			return exitCode, err
		}
		return 0, nil
	}
	fileURI, err := r.remoteArchiveFolder(r.Project, r.Project.GetRootDir(), r.Project.GetSauceCfg().Sauceignore)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(fileURI)
	if passed {
		exitCode = 0
	}

	if deprecationMessage != "" {
		fmt.Print(deprecationMessage)
	}

	return exitCode, nil
}

// checkCypressVersion do several checks before running Cypress tests.
func (r *CypressRunner) checkCypressVersion() error {
	if r.Project.GetVersion() == "" {
		return fmt.Errorf("missing cypress version. Check available versions here: https://docs.saucelabs.com/dev/cli/saucectl/#supported-frameworks-and-browsers")
	}
	return nil
}

func (r *CypressRunner) runSuites(fileURI string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)
	jobOpts, results, err := r.createWorkerPool(r.Project.GetSauceCfg().Concurrency, r.Project.GetSauceCfg().Retries)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on.
	go func() {
		for _, s := range r.Project.GetSuites() {
			jobOpts <- job.StartOptions{
				ConfigFilePath:   r.Project.GetCfgPath(),
				CLIFlags:         r.Project.GetCLIFlags(),
				DisplayName:      s.Name,
				Timeout:          s.Timeout,
				App:              fileURI,
				Suite:            s.Name,
				Framework:        "cypress",
				FrameworkVersion: r.Project.GetVersion(),
				BrowserName:      s.Browser,
				BrowserVersion:   s.BrowserVersion,
				PlatformName:     s.PlatformName,
				Name:             s.Name,
				Build:            r.Project.GetSauceCfg().Metadata.Build,
				Tags:             r.Project.GetSauceCfg().Metadata.Tags,
				Tunnel: job.TunnelOptions{
					ID:     r.Project.GetSauceCfg().Tunnel.Name,
					Parent: r.Project.GetSauceCfg().Tunnel.Owner,
				},
				ScreenResolution: s.ScreenResolution,
				RunnerVersion:    r.Project.GetRunnerVersion(),
				Experiments:      r.Project.GetSauceCfg().Experiments,
				Attempt:          0,
				Retries:          r.Project.GetSauceCfg().Retries,
				TimeZone:         s.TimeZone,
			}
		}
	}()

	return r.collectResults(r.Project.GetArtifactsCfg().Download, results, r.Project.GetSuiteCount())
}
