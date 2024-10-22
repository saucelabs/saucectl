package saucecloud

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
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
	armRequired     bool
}

// EspressoRunner represents the Sauce Labs cloud implementation for cypress.
type EspressoRunner struct {
	CloudRunner
	Project espresso.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *EspressoRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(
		r.Project.Sauce.Tunnel.Name,
		r.Project.Sauce.Tunnel.Owner,
		r.Project.DryRun,
		r.Project.Sauce.Tunnel.Timeout,
	); err != nil {
		return 1, err
	}

	var err error
	r.Project.Espresso.App, err = r.uploadProject(r.Project.Espresso.App, r.Project.Espresso.AppDescription, appUpload, r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	r.Project.Espresso.OtherApps, err = r.uploadProjects(r.Project.Espresso.OtherApps, otherAppsUpload, r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	cache := map[string]string{}
	for i, suite := range r.Project.Suites {
		if val, ok := cache[suite.TestApp]; ok {
			r.Project.Suites[i].TestApp = val
			continue
		}

		testAppURL, err := r.uploadProject(suite.TestApp, suite.TestAppDescription, testAppUpload, r.Project.DryRun)
		if err != nil {
			return exitCode, err
		}
		r.Project.Suites[i].TestApp = testAppURL
		cache[suite.TestApp] = testAppURL
	}

	if r.Project.DryRun {
		r.dryRun()
		return 0, nil
	}

	passed := r.runSuites()
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *EspressoRunner) runSuites() bool {
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
			suites = espresso.SortByHistory(suites, history)
		}
	}
	// Submit suites to work on.
	jobsCount := r.calculateJobsCount(suites)
	go func() {
		for _, s := range suites {
			shardCfg := s.ShardConfig()
			// Automatically apply ShardIndex if numShards is defined
			if shardCfg.Shards > 0 {
				for i := 0; i < shardCfg.Shards; i++ {
					// Enforce copy of the map to ensure it is not shared.
					testOptions := map[string]interface{}{}
					for k, v := range s.TestOptions {
						testOptions[k] = v
					}
					s.TestOptions = testOptions
					s.TestOptions["shardIndex"] = i
					for _, c := range enumerateDevices(s.Devices, s.Emulators) {
						log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
						r.startJob(jobOpts, s, r.Project.Espresso.App, s.TestApp, r.Project.Espresso.OtherApps, c)
					}
				}
			} else {
				for _, c := range enumerateDevices(s.Devices, s.Emulators) {
					log.Debug().Str("suite", s.Name).Str("device", fmt.Sprintf("%v", c)).Msg("Starting job")
					r.startJob(jobOpts, s, r.Project.Espresso.App, s.TestApp, r.Project.Espresso.OtherApps, c)
				}
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

func (r *EspressoRunner) dryRun() {
	fmt.Println("\nThe following test suites would have run:")
	for _, s := range r.Project.Suites {
		fmt.Printf("  - %s\n", s.Name)
		for _, c := range enumerateDevices(s.Devices, s.Emulators) {
			fmt.Printf("    - on %s %s %s\n", c.name, c.platformName, c.platformVersion)
		}
	}
	fmt.Println()
}

// enumerateDevices returns a list of emulators and devices targeted by the current suite.
func enumerateDevices(devices []config.Device, virtualDevices []config.VirtualDevice) []deviceConfig {
	var configs []deviceConfig

	for _, e := range virtualDevices {
		for _, p := range e.PlatformVersions {
			configs = append(configs, deviceConfig{
				name:            e.Name,
				platformName:    e.PlatformName,
				platformVersion: p,
				orientation:     e.Orientation,
				armRequired:     e.ARMRequired,
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
func (r *EspressoRunner) startJob(jobOpts chan<- job.StartOptions, s espresso.Suite, appFileURI, testAppFileURI string, otherAppsURIs []string, d deviceConfig) {
	displayName := s.Name
	shardCfg := s.ShardConfig()
	if shardCfg.Shards > 0 {
		displayName = fmt.Sprintf("%s (shard %d/%d)", displayName, shardCfg.Index+1, shardCfg.Shards)
	}

	jobOpts <- job.StartOptions{
		DisplayName:       displayName,
		Timeout:           s.Timeout,
		ConfigFilePath:    r.Project.ConfigFilePath,
		CLIFlags:          r.Project.CLIFlags,
		App:               appFileURI,
		TestApp:           testAppFileURI,
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
			Name:  r.Project.Sauce.Tunnel.Name,
			Owner: r.Project.Sauce.Tunnel.Owner,
		},
		Experiments:   r.Project.Sauce.Experiments,
		TestOptions:   s.TestOptions,
		Attempt:       0,
		Retries:       r.Project.Sauce.Retries,
		Visibility:    r.Project.Sauce.Visibility,
		PassThreshold: s.PassThreshold,
		SmartRetry: job.SmartRetry{
			FailedOnly: s.SmartRetry.IsRetryFailedOnly(),
		},

		// RDC Specific flags
		RealDevice:        d.isRealDevice,
		DeviceHasCarrier:  d.hasCarrier,
		DeviceType:        d.deviceType,
		DevicePrivateOnly: d.privateOnly,

		// Overwrite device settings
		RealDeviceKind: strings.ToLower(espresso.Android),
		AppSettings: job.AppSettings{
			AudioCapture: s.AppSettings.AudioCapture,
			Instrumentation: job.Instrumentation{
				NetworkCapture: s.AppSettings.Instrumentation.NetworkCapture,
			},
		},
	}
}

func (r *EspressoRunner) calculateJobsCount(suites []espresso.Suite) int {
	total := 0
	for _, s := range suites {
		jobs := len(enumerateDevices(s.Devices, s.Emulators))
		shardCfg := s.ShardConfig()
		if shardCfg.Shards > 0 {
			jobs *= shardCfg.Shards
		}
		total += jobs
	}
	return total
}
