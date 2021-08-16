package saucecloud

import (
	"fmt"
	"strings"

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

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID, r.Project.Sauce.Tunnel.Parent); err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		if err := r.dryRun(r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore, r.getSuiteNames()); err != nil {
			return exitCode, err
		}
		return 0, nil
	}

	fileID, err := r.archiveAndUpload(r.Project, r.Project.RootDir, r.Project.Sauce.Sauceignore)
	if err != nil {
		return exitCode, err
	}
	passed := r.runSuites(fileID)
	if passed {
		return 0, nil
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

func (r *TestcafeRunner) runSuites(fileID string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on
	jobsCount := r.calcTestcafeJobsCount(r.Project.Suites)
	go func() {
		for _, s := range r.Project.Suites {
			if len(s.Simulators) > 0 {
				for _, d := range s.Simulators {
					for _, pv := range d.PlatformVersions {
						jobOpts <- job.StartOptions{
							ConfigFilePath:   r.Project.ConfigFilePath,
							CLIFlags:         r.Project.CLIFlags,
							DisplayName:      s.Name,
							Timeout:          s.Timeout,
							App:              fmt.Sprintf("storage:%s", fileID),
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
								ID:     r.Project.Sauce.Tunnel.ID,
								Parent: r.Project.Sauce.Tunnel.Parent,
							},
							ScreenResolution: s.ScreenResolution,
							RunnerVersion:    r.Project.RunnerVersion,
							Experiments:      r.Project.Sauce.Experiments,
						}
					}
				}
			} else {
				jobOpts <- job.StartOptions{
					ConfigFilePath:   r.Project.ConfigFilePath,
					DisplayName:      s.Name,
					App:              fmt.Sprintf("storage:%s", fileID),
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
						ID:     r.Project.Sauce.Tunnel.ID,
						Parent: r.Project.Sauce.Tunnel.Parent,
					},
					ScreenResolution: s.ScreenResolution,
					RunnerVersion:    r.Project.RunnerVersion,
					Experiments:      r.Project.Sauce.Experiments,
					Attempt:          0,
					Retries:          r.Project.Sauce.Retries,
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
