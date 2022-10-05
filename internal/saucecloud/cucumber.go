package saucecloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cucumber"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"

	"github.com/saucelabs/saucectl/internal/job"
)

// CucumberRunner represents the SauceLabs cloud implementation
type CucumberRunner struct {
	CloudRunner
	Project cucumber.Project
}

// RunProject runs the defined tests on sauce cloud
func (r *CucumberRunner) RunProject() (int, error) {
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

	if m.Deprecated {
		deprecationMessage = r.deprecationMessage(playwright.Kind, r.Project.Playwright.Version)
		fmt.Print(deprecationMessage)
	}
	for _, s := range r.Project.Suites {
		if s.PlatformName != "" && !framework.HasPlatform(m, s.PlatformName) {
			msg.LogUnsupportedPlatform(s.PlatformName, framework.PlatformNames(m.Platforms))
			return 1, errors.New("unsupported platform")
		}
	}

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner, r.Project.DryRun); err != nil {
		return 1, err
	}

	fileURI, err := r.remoteArchiveProject(r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore, r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	if r.Project.DryRun {
		log.Info().Msgf("The following test suites would have run: [%s].", r.getSuiteNames())
		return 0, nil
	}

	passed := r.runSuites(fileURI)
	if passed {
		return 0, nil
	}

	if deprecationMessage != "" {
		fmt.Print(deprecationMessage)
	}

	return exitCode, nil
}

func (r *CucumberRunner) getSuiteNames() string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}

	return strings.Join(names, ", ")
}

func (r *CucumberRunner) runSuites(fileURI string) bool {
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
				App:              fmt.Sprintf("storage:%s", fileURI),
				Suite:            s.Name,
				Framework:        "playwright",
				FrameworkVersion: r.Project.Playwright.Version,
				BrowserName:      s.BrowserName,
				BrowserVersion:   s.BrowserVersion,
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
				Visibility:       r.Project.Sauce.Visibility,
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, len(r.Project.Suites))
}
