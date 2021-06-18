package saucecloud

import (
	"fmt"
	"strings"
	"context"
	"time"

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
	isRealDevice    bool
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

	if r.DryRun {
		r.dryRun()
		return 0, nil
	}

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
			for _, c := range enumerateDevicesAndEmulators(s.Devices, s.Emulators) {
				if(c.isRealDevice && strings.Contains(c.ID, ",")) {
					// if , detected in device names, ping rdc for availability before launching.
					log.Debug().Str("device", fmt.Sprintf("%v", c)).Msg("Looking for available device")
					j, _ := r.RDCJobReader.PollDevicesState(context.Background(), c.ID, 15*time.Second)
					c.ID = j
				}

				log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
				r.startJob(jobOpts, s, appFileID, testAppFileID, c)
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

func (r *EspressoRunner) dryRun() {
	log.Warn().Msg("Running tests in dry run mode.")
	for _, s := range r.Project.Suites {
		for _, c := range enumerateDevicesAndEmulators(s.Devices, s.Emulators) {
			log.Info().Msgf("The [%s] suite would run on %s %s %s.", s.Name, c.name, c.platformName, c.platformVersion)
		}
	}
}

// enumerateDevicesAndEmulators returns a list of emulators and devices targeted by the current suite.
func enumerateDevicesAndEmulators(devices []config.Device, emulators []config.Emulator) []deviceConfig {
	var configs []deviceConfig

	for _, e := range emulators {
		for _, p := range e.PlatformVersions {
			configs = append(configs, deviceConfig{
				name:            e.Name,
				platformName:    e.PlatformName,
				platformVersion: p,
				orientation:     e.Orientation,
			})
		}
	}

	for _, d := range devices {
		configs = append(configs, deviceConfig{
			ID:              d.ID,
			name:            d.Name,
			platformName:    d.PlatformName,
			platformVersion: d.PlatformVersion,
			isRealDevice:    true,
			hasCarrier:      d.Options.CarrierConnectivity,
			deviceType:      d.Options.DeviceType,
			privateOnly:     d.Options.Private,
		})
	}
	return configs
}

// startJob add the job to the list for the workers.
func (r *EspressoRunner) startJob(jobOpts chan<- job.StartOptions, s espresso.Suite, appFileID, testAppFileID string, d deviceConfig) {
	jto := job.TestOptions{
		NotClass:   s.TestOptions.NotClass,
		Class:      s.TestOptions.Class,
		Annotation: s.TestOptions.Annotation,
		Size:       s.TestOptions.Size,
		Package:    s.TestOptions.Package,
	}
	if s.TestOptions.NumShards > 1 {
		jto.NumShards = &s.TestOptions.NumShards
		jto.ShardIndex = &s.TestOptions.ShardIndex
	}
	if s.TestOptions.ClearPackageData {
		jto.ClearPackageData = &s.TestOptions.ClearPackageData
	}

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
		Name:              s.Name,
		Build:             r.Project.Sauce.Metadata.Build,
		Tags:              r.Project.Sauce.Metadata.Tags,
		Tunnel: job.TunnelOptions{
			ID:     r.Project.Sauce.Tunnel.ID,
			Parent: r.Project.Sauce.Tunnel.Parent,
		},
		Experiments: r.Project.Sauce.Experiments,
		TestOptions: jto,

		// RDC Specific flags
		RealDevice:        d.isRealDevice,
		DeviceHasCarrier:  d.hasCarrier,
		DeviceType:        d.deviceType,
		DevicePrivateOnly: d.privateOnly,
	}
}

func (r *EspressoRunner) calculateJobsCount(suites []espresso.Suite) int {
	jobsCount := 0
	for _, s := range suites {
		jobsCount += len(enumerateDevicesAndEmulators(s.Devices, s.Emulators))
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
