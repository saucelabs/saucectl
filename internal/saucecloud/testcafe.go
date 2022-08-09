package saucecloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"

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
	var deprecationMessage string
	exitCode := 1

	m, err := r.MetadataSearchStrategy.Find(context.Background(), r.MetadataService, testcafe.Kind, r.Project.Testcafe.Version)
	if err != nil {
		r.logFrameworkError(err)
		return exitCode, err
	}
	r.Project.Testcafe.Version = m.FrameworkVersion
	if r.Project.RunnerVersion == "" {
		r.Project.RunnerVersion = m.CloudRunnerVersion
	}

	if m.Deprecated {
		deprecationMessage = r.deprecationMessage(testcafe.Kind, r.Project.Testcafe.Version)
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

func (r *TestcafeRunner) getSuiteNames() string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}

	return strings.Join(names, ", ")
}

func (r *TestcafeRunner) runSuites(fileURI string) bool {
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
						jobOpts <- job.StartOptions{
							ConfigFilePath:   r.Project.ConfigFilePath,
							CLIFlags:         r.Project.CLIFlags,
							DisplayName:      s.Name,
							Timeout:          s.Timeout,
							App:              fileURI,
							Suite:            s.Name,
							Framework:        "testcafe",
							FrameworkVersion: r.Project.Testcafe.Version,
							BrowserName:      s.BrowserName,
							BrowserVersion:   s.BrowserVersion,
							PlatformName:     d.PlatformName,
							PlatformVersion:  pv,
							DeviceName:       d.Name,
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
						}
					}
				}
			} else {
				jobOpts <- job.StartOptions{
					ConfigFilePath:   r.Project.ConfigFilePath,
					DisplayName:      s.Name,
					App:              fmt.Sprintf("storage:%s", fileURI),
					Suite:            s.Name,
					Framework:        "testcafe",
					FrameworkVersion: r.Project.Testcafe.Version,
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
					TimeZone:         s.TimeZone,
					Visibility:       r.Project.Sauce.Visibility,
				}
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
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
