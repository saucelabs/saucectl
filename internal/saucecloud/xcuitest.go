package saucecloud

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

// XcuitestRunner represents the Sauce Labs cloud implementation for xcuitest.
type XcuitestRunner struct {
	CloudRunner
	Project xcuitest.Project
}

// RunProject runs the tests defined in xcuitest.Project.
func (r *XcuitestRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return exitCode, err
	}

	appFileID, err := r.uploadProject(r.Project.Xcuitest.App, appUpload)
	if err != nil {
		return exitCode, err
	}

	testAppFileID, err := r.uploadProject(r.Project.Xcuitest.TestApp, testAppUpload)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(appFileID, testAppFileID)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *XcuitestRunner) runSuites(appFileID, testAppFileID string) bool {
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
				log.Debug().Str("suite", s.Name).Str("deviceName", d.Name).Str("deviceID", d.ID).Str("platformVersion", d.PlatformVersion).Msg("Starting job")
				r.startJob(jobOpts, appFileID, testAppFileID, s, d)
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

func (r *XcuitestRunner) startJob(jobOpts chan<- job.StartOptions, appFileID, testAppFileID string, s xcuitest.Suite, d config.Device) {
	jobOpts <- job.StartOptions{
		ConfigFilePath:   r.Project.ConfigFilePath,
		DisplayName:      s.Name,
		App:              fmt.Sprintf("storage:%s", appFileID),
		Suite:            fmt.Sprintf("storage:%s", testAppFileID),
		Framework:        "xcuitest",
		FrameworkVersion: "1.0.0-stable",
		PlatformName:     d.PlatformName,
		PlatformVersion:  d.PlatformVersion,
		DeviceName:       d.Name,
		DeviceID:         d.ID,
		Name:             r.Project.Sauce.Metadata.Name + " - " + s.Name,
		Build:            r.Project.Sauce.Metadata.Build,
		Tags:             r.Project.Sauce.Metadata.Tags,
		Tunnel: job.TunnelOptions{
			ID:     r.Project.Sauce.Tunnel.ID,
			Parent: r.Project.Sauce.Tunnel.Parent,
		},
		Experiments: r.Project.Sauce.Experiments,
		TestOptions: job.TestOptions{
			Class: s.TestOptions.Class,
		},

		// RDC Specific flags
		RealDevice:        true,
		DeviceHasCarrier:  d.Options.CarrierConnectivity,
		DeviceType:        d.Options.DeviceType,
		DevicePrivateOnly: d.Options.Private,
	}
}

func (r *XcuitestRunner) calculateJobsCount(suites []xcuitest.Suite) int {
	jobsCount := 0
	for _, s := range suites {
		jobsCount += len(s.Devices)
	}
	return jobsCount
}
