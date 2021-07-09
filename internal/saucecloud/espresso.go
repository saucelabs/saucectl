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

	var otherAppsIDs []string
	for _, o := range r.Project.Espresso.OtherApps {
		ID, err := r.uploadProject(o, otherAppsUpload)
		if err != nil {
			return exitCode, err
		}
		otherAppsIDs = append(otherAppsIDs, fmt.Sprintf("storage:%s", ID))
	}

	passed := r.runSuites(appFileID, testAppFileID, otherAppsIDs)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *EspressoRunner) runSuites(appFileID string, testAppFileID string, otherAppsIDs []string) bool {
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
			// Automatically apply ShardIndex if not specified
			if s.TestOptions.NumShards > 0 {
				for i := 0; i < s.TestOptions.NumShards; i++ {
					s.TestOptions.ShardIndex = i
					for _, c := range enumerateDevicesAndEmulators(s.Devices, s.Emulators) {
						log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
						r.startJob(jobOpts, s, appFileID, testAppFileID, otherAppsIDs, c)
					}
				}
			} else {
				for _, c := range enumerateDevicesAndEmulators(s.Devices, s.Emulators) {
					log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
					r.startJob(jobOpts, s, appFileID, testAppFileID, otherAppsIDs, c)
				}
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
func (r *EspressoRunner) startJob(jobOpts chan<- job.StartOptions, s espresso.Suite, appFileID, testAppFileID string, otherAppsIDs []string, d deviceConfig) {
	jto := job.TestOptions{
		NotClass:   s.TestOptions.NotClass,
		Class:      s.TestOptions.Class,
		Annotation: s.TestOptions.Annotation,
		Size:       s.TestOptions.Size,
		Package:    s.TestOptions.Package,
	}
	displayName := s.Name
	if s.TestOptions.NumShards > 0 {
		jto.NumShards = &s.TestOptions.NumShards
		jto.ShardIndex = &s.TestOptions.ShardIndex
		displayName = fmt.Sprintf("%s (shard %d/%d)", displayName, *jto.ShardIndex+1, *jto.NumShards)
	}
	if s.TestOptions.ClearPackageData {
		jto.ClearPackageData = &s.TestOptions.ClearPackageData
	}

	jobOpts <- job.StartOptions{
		DisplayName:       displayName,
		ConfigFilePath:    r.Project.ConfigFilePath,
		App:               fmt.Sprintf("storage:%s", appFileID),
		Suite:             fmt.Sprintf("storage:%s", testAppFileID),
		OtherApps:         otherAppsIDs,
		Framework:         "espresso",
		FrameworkVersion:  "1.0.0-stable",
		PlatformName:      d.platformName,
		PlatformVersion:   d.platformVersion,
		DeviceID:          d.ID,
		DeviceName:        d.name,
		DeviceOrientation: d.orientation,
		Name:              displayName,
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
	total := 0
	for _, s := range suites {
		jobs := len(enumerateDevicesAndEmulators(s.Devices, s.Emulators))
		if s.TestOptions.NumShards > 0 {
			jobs *= s.TestOptions.NumShards
		}
		total += jobs
	}
	return total
}

func (r *EspressoRunner) getSuiteNames() string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}

	return strings.Join(names, ", ")
}
