package saucecloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/storage"
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
func (r *EspressoRunner) RunProject(ctx context.Context) (int, error) {
	exitCode := 1

	if err := r.validateTunnel(
		ctx,
		r.Project.Sauce.Tunnel.Name,
		r.Project.Sauce.Tunnel.Owner,
		r.Project.DryRun,
		r.Project.Sauce.Tunnel.Timeout,
	); err != nil {
		return 1, err
	}

	var err error
	r.Project.Espresso.App, err = r.uploadArchive(
		ctx,
		storage.FileInfo{Name: r.Project.Espresso.App, Description: r.Project.Espresso.AppDescription},
		appUpload,
		r.Project.DryRun,
	)
	if err != nil {
		return exitCode, err
	}

	r.Project.Espresso.OtherApps, err = r.uploadArchives(ctx, r.Project.Espresso.OtherApps, otherAppsUpload, r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	cache := map[string]string{}
	for i, suite := range r.Project.Suites {
		if val, ok := cache[suite.TestApp]; ok {
			r.Project.Suites[i].TestApp = val
			continue
		}

		testAppURL, err := r.uploadArchive(
			ctx,
			storage.FileInfo{Name: suite.TestApp, Description: suite.TestAppDescription},
			testAppUpload,
			r.Project.DryRun,
		)
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

	passed := r.runSuites(ctx)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *EspressoRunner) runSuites(ctx context.Context) bool {
	jobOpts, results := r.createWorkerPool(
		ctx, r.Project.Sauce.Concurrency, r.Project.Sauce.Retries,
	)
	defer close(results)

	suites := r.Project.Suites
	if r.Project.Sauce.LaunchOrder != "" {
		history, err := r.getHistory(ctx, r.Project.Sauce.LaunchOrder)
		if err != nil {
			log.Warn().Err(err).Msg(msg.RetrieveJobHistoryError)
		} else {
			suites = espresso.SortByHistory(suites, history)
		}
	}

	var startOptions []job.StartOptions
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
				for _, deviceCfg := range enumerateDevices(
					s.Devices, s.Emulators,
				) {
					startOptions = append(
						startOptions, r.newStartOptions(
							s, r.Project.Espresso.App, s.TestApp,
							r.Project.Espresso.OtherApps, deviceCfg,
						),
					)
				}
			}
		} else {
			for _, deviceCfg := range enumerateDevices(s.Devices, s.Emulators) {
				startOptions = append(
					startOptions, r.newStartOptions(
						s, r.Project.Espresso.App, s.TestApp,
						r.Project.Espresso.OtherApps, deviceCfg,
					),
				)
			}
		}
	}

	go func() {
		for _, opt := range startOptions {
			jobOpts <- opt
		}
	}()

	return r.collectResults(ctx, results, len(startOptions))
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

// newStartOptions add the job to the list for the workers.
func (r *EspressoRunner) newStartOptions(
	s espresso.Suite, appFileURI, testAppFileURI string, otherAppsURIs []string,
	d deviceConfig,
) job.StartOptions {
	displayName := s.Name
	shardCfg := s.ShardConfig()
	if shardCfg.Shards > 0 {
		displayName = fmt.Sprintf(
			"%s (shard %d/%d)", displayName, shardCfg.Index+1, shardCfg.Shards,
		)
	}

	return job.StartOptions{
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

		// Configure device settings
		RealDeviceKind: strings.ToLower(espresso.Android),
		AppSettings: job.AppSettings{
			////////////////////////////////////////////////////////////////////////////////////////////////////////////
			// The key name mismatch here between left and right hand side is intentional.
			//
			//   Traditionally, saucectl made no distinction between instrumentation and resigning
			//   while our composer API backend always used specific key names per platform,
			//   either Instrumentation for Android and Resigning for iOS.
			//
			//   This created a situation where app settings were sporadically ignored in Android RDC sessions.
			//
			//   Here, we choose the lesser evil and keep supporting  `ResignerEnabled` in the saucectl config yaml
			//   for backward compatibility while also mapping it to the correct API parameter for composer to evaluate.
			InstrumentationEnabled: s.AppSettings.ResigningEnabled,
			////////////////////////////////////////////////////////////////////////////////////////////////////////////

			AudioCapture: s.AppSettings.AudioCapture,
			Instrumentation: job.Instrumentation{
				ImageInjection:              s.AppSettings.Instrumentation.ImageInjection,
				BypassScreenshotRestriction: s.AppSettings.Instrumentation.BypassScreenshotRestriction,
				Vitals:                      s.AppSettings.Instrumentation.Vitals,
				NetworkCapture:              s.AppSettings.Instrumentation.NetworkCapture,
				BiometricsInterception:      s.AppSettings.Instrumentation.Biometrics,
			},
		},
	}
}
