package saucecloud

import (
	"github.com/saucelabs/saucectl/internal/vmrunner"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/testcafe"
)

// VMRunner represents the SauceLabs cloud implementation
type VMRunner struct {
	CloudRunner
	Project vmrunner.Project
}

// RunProject runs the defined tests on sauce cloud
func (r *VMRunner) RunProject() (int, error) {
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

func (r *VMRunner) getSuiteNames() []string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return names
}

func (r *VMRunner) runSuites(app string, otherApps []string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency, r.Project.Sauce.Retries)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on
	go func() {
		for _, s := range r.Project.Suites {
			opts := r.generateStartOpts(s)
			opts.App = app
			opts.OtherApps = otherApps
			opts.PlatformName = s.Platform
			jobOpts <- opts
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, len(r.Project.Suites))
}

func (r *VMRunner) generateStartOpts(s vmrunner.Suite) job.StartOptions {
	return job.StartOptions{
		ConfigFilePath:   r.Project.ConfigFilePath,
		DisplayName:      s.Name,
		Timeout:          s.Timeout,
		Suite:            s.Name,
		Framework:        r.Project.Workload,
		FrameworkVersion: "1.0.0",
		BrowserName:      s.Browser,
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
		PassThreshold:    1,
	}
}

func (r *VMRunner) calcTestcafeJobsCount(suites []testcafe.Suite) int {
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
