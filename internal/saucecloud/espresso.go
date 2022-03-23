package saucecloud

import (
	"fmt"
	"strconv"
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

	if r.Project.DryRun {
		r.dryRun()
		return 0, nil
	}

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner); err != nil {
		return 1, err
	}

	appFileURI, err := r.uploadProject(r.Project.Espresso.App, appUpload)
	if err != nil {
		return exitCode, err
	}

	testAppFileURI, err := r.uploadProject(r.Project.Espresso.TestApp, testAppUpload)
	if err != nil {
		return exitCode, err
	}

	otherAppsURIs, err := r.uploadProjects(r.Project.Espresso.OtherApps, otherAppsUpload)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(appFileURI, testAppFileURI, otherAppsURIs)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *EspressoRunner) runSuites(appFileURI string, testAppFileURI string, otherAppsURIs []string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency, r.Project.Sauce.Retries)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on.
	jobsCount := r.calculateJobsCount(r.Project.Suites)
	go func() {
		for _, s := range r.Project.Suites {
			numShards, _ := getNumShardsAndShardIndex(s.TestOptions)
			// Automatically apply ShardIndex if numShards is defined
			if numShards > 0 {
				for i := 0; i < numShards; i++ {
					// Enforce copy of the map to ensure it is not shared.
					testOptions := map[string]interface{}{}
					for k, v := range s.TestOptions {
						testOptions[k] = v
					}
					s.TestOptions = testOptions
					s.TestOptions["shardIndex"] = i
					for _, c := range enumerateDevicesAndEmulators(s.Devices, s.Emulators) {
						log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
						r.startJob(jobOpts, s, appFileURI, testAppFileURI, otherAppsURIs, c)
					}
				}
			} else {
				for _, c := range enumerateDevicesAndEmulators(s.Devices, s.Emulators) {
					log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
					r.startJob(jobOpts, s, appFileURI, testAppFileURI, otherAppsURIs, c)
				}
			}
		}
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

// getNumShardsAndShardIndex extracts numShards and shardIndex from testOptions.
func getNumShardsAndShardIndex(testOptions map[string]interface{}) (int, int) {
	outNumShards := 0
	outShardIndex := 0
	numShards, hasNumShards := testOptions["numShards"]
	shardIndex, hasShardIndex := testOptions["shardIndex"]
	if hasNumShards {
		if v, err := strconv.Atoi(fmt.Sprintf("%v", numShards)); err == nil {
			outNumShards = v
		}
	}
	if hasShardIndex {
		if v, err := strconv.Atoi(fmt.Sprintf("%v", shardIndex)); err == nil {
			outShardIndex = v
		}
	}
	return outNumShards, outShardIndex
}

// startJob add the job to the list for the workers.
func (r *EspressoRunner) startJob(jobOpts chan<- job.StartOptions, s espresso.Suite, appFileURI, testAppFileURI string, otherAppsURIs []string, d deviceConfig) {
	displayName := s.Name
	numShards, shardIndex := getNumShardsAndShardIndex(s.TestOptions)
	if numShards > 0 {
		displayName = fmt.Sprintf("%s (shard %d/%d)", displayName, shardIndex+1, numShards)
	}

	jobOpts <- job.StartOptions{
		DisplayName:       displayName,
		Timeout:           s.Timeout,
		ConfigFilePath:    r.Project.ConfigFilePath,
		CLIFlags:          r.Project.CLIFlags,
		App:               appFileURI,
		Suite:             testAppFileURI,
		OtherApps:         otherAppsURIs,
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
			ID:     r.Project.Sauce.Tunnel.Name,
			Parent: r.Project.Sauce.Tunnel.Owner,
		},
		Experiments: r.Project.Sauce.Experiments,
		TestOptions: s.TestOptions,
		Attempt:     0,
		Retries:     r.Project.Sauce.Retries,

		// RDC Specific flags
		RealDevice:        d.isRealDevice,
		DeviceHasCarrier:  d.hasCarrier,
		DeviceType:        d.deviceType,
		DevicePrivateOnly: d.privateOnly,

		// Instrumentation options, for RDC only
		AudioCapture:   r.Project.Instrumentation.AudioCapture,
		NetworkCapture: r.Project.Instrumentation.NetworkCapture,
	}
}

func (r *EspressoRunner) calculateJobsCount(suites []espresso.Suite) int {
	total := 0
	for _, s := range suites {
		jobs := len(enumerateDevicesAndEmulators(s.Devices, s.Emulators))
		numShards, _ := getNumShardsAndShardIndex(s.TestOptions)
		if numShards > 0 {
			jobs *= numShards
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
