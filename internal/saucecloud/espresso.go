package saucecloud

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
)

// deviceConfig represent the configuration for a specific device.
type deviceConfig struct {
	ID              string
	name            string
	platformName    string
	platformVersion string
	orientation     string
	hasCarrier      bool
	deviceType      string
	privateOnly     bool
}

// EspressoRunner represents the Sauce Labs cloud implementation for cypress.
type EspressoRunner struct {
	CloudRunner
	Project espresso.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *EspressoRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return 1, err
	}

	appFileID, err := r.uploadProject(r.Project.Espresso.App, appUpload)
	if err != nil {
		return exitCode, err
	}

	testAppFileID, err := r.uploadProject(r.Project.Espresso.TestApp, testAppUpload)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(appFileID, testAppFileID)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *EspressoRunner) runSuites(appFileID string, testAppFileID string) bool {
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
				for _, c := range enumerateDevices(d) {
					log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
					r.startJob(jobOpts, s, appFileID, testAppFileID, c)
				}
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

// enumerateDevices returns a list of device targeted by the current suite.
func enumerateDevices(d config.Device) []deviceConfig {
	if d.ID != "" {
		return []deviceConfig{{ID: d.ID, platformName: d.PlatformName}}
	}
	var configs []deviceConfig
	for _, p := range d.PlatformVersions {
		configs = append(configs, deviceConfig{
			name:            d.Name,
			platformName:    d.PlatformName,
			platformVersion: p,
			orientation:     d.Orientation,
		})
	}
	return configs
}

// startJob add the job to the list for the workers.
func (r *EspressoRunner) startJob(jobOpts chan<- job.StartOptions, s espresso.Suite, appFileID, testAppFileID string, d deviceConfig) {
	jobOpts <- job.StartOptions{
		DisplayName:       s.Name,
		ConfigFilePath:    r.Project.ConfigFilePath,
		App:               fmt.Sprintf("storage:%s", appFileID),
		Suite:             fmt.Sprintf("storage:%s", testAppFileID),
		Framework:         "espresso",
		FrameworkVersion:  "1.0.0-stable",
		PlatformName:      d.platformName,
		PlatformVersion:   d.platformVersion,
		DeviceID:          d.ID,
		DeviceName:        d.name,
		DeviceOrientation: d.orientation,
		Name:              r.Project.Sauce.Metadata.Name + " - " + s.Name,
		Build:             r.Project.Sauce.Metadata.Build,
		Tags:              r.Project.Sauce.Metadata.Tags,
		Tunnel: job.TunnelOptions{
			ID:     r.Project.Sauce.Tunnel.ID,
			Parent: r.Project.Sauce.Tunnel.Parent,
		},
		Experiments: r.Project.Sauce.Experiments,
		TestOptions: job.TestOptions{
			NotClass:   s.TestOptions.NotClass,
			Class:      s.TestOptions.Class,
			Annotation: s.TestOptions.Annotation,
			Size:       s.TestOptions.Size,
			Package:    s.TestOptions.Package,
		},

		// RDC Specific flags
		DeviceHasCarrier:  d.hasCarrier,
		DeviceType:        d.deviceType,
		DevicePrivateOnly: d.privateOnly,
	}
}

func (r *EspressoRunner) calculateJobsCount(suites []espresso.Suite) int {
	jobsCount := 0
	for _, s := range suites {
		for _, d := range s.Devices {
			jobsCount += len(enumerateDevices(d))
		}
	}
	return jobsCount
}

func (r *EspressoRunner) getSuiteNames() string {
	names := []string{}
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}

	return strings.Join(names, ", ")
}
