package saucecloud

import (
	"fmt"
	"strings"

	"github.com/saucelabs/saucectl/internal/xcuit"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
)

// XcuitRunner represents the Sauce Labs cloud implementation for xcuit.
type XcuitRunner struct {
	CloudRunner
	Project xcuit.Project
}

// RunProject runs the tests defined in xcuit.Project.
func (r *XcuitRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return 1, err
	}

	appFileID, err := r.uploadProject(r.Project.Xcuit.App, appUpload)
	if err != nil {
		return exitCode, err
	}

	testAppFileID, err := r.uploadProject(r.Project.Xcuit.TestApp, testAppUpload)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(appFileID, testAppFileID)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *XcuitRunner) runSuites(appFileID string, testAppFileID string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on.
	jobsCount := r.calculateJobsCount(r.Project.Suites)
	go func() {
		for _, s := range r.Project.Suites {
			for _, d := range s.Devices {
				log.Debug().Str("suite", s.Name).Str("device", d.Name).Str("platformVersion", d.PlatformVersion).Msg("Starting job")
				jobOpts <- job.StartOptions{
					ConfigFilePath: r.Project.ConfigFilePath,
					DisplayName:    s.Name,
					App:            fmt.Sprintf("storage:%s", appFileID),
					Suite:          fmt.Sprintf("storage:%s", testAppFileID),
					Framework:      "xcuit",
					DeviceName:     d.Name,
					Name:           r.Project.Sauce.Metadata.Name + " - " + s.Name,
					Build:          r.Project.Sauce.Metadata.Build,
					Tags:           r.Project.Sauce.Metadata.Tags,
					Tunnel: job.TunnelOptions{
						ID:     r.Project.Sauce.Tunnel.ID,
						Parent: r.Project.Sauce.Tunnel.Parent,
					},
					Experiments: r.Project.Sauce.Experiments,
					TestOptions: job.TestOptions{
						Class: s.TestOptions.Class,
					},
				}
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

func (r *XcuitRunner) calculateJobsCount(suites []xcuit.Suite) int {
	jobsCount := 0
	for _, s := range suites {
		jobsCount += len(s.Devices)
	}
	return jobsCount
}

func (r *XcuitRunner) getSuiteNames() string {
	names := []string{}
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}

	return strings.Join(names, ", ")
}
