package saucecloud

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

// XcuitestRunner represents the Sauce Labs cloud implementation for xcuitest.
type XcuitestRunner struct {
	CloudRunner
	Project xcuitest.Project
}

type archiveType string

var (
	ipaArchive archiveType = "ipa"
	zipArchive archiveType = "zip"
)

// RunProject runs the tests defined in xcuitest.Project.
func (r *XcuitestRunner) RunProject(ctx context.Context) (int, error) {
	exitCode := 1

	if err := r.validateTunnel(
		ctx,
		r.Project.Sauce.Tunnel.Name,
		r.Project.Sauce.Tunnel.Owner,
		r.Project.DryRun,
		r.Project.Sauce.Tunnel.Timeout,
	); err != nil {
		return exitCode, err
	}

	archiveCache := newCache()
	uploadCache := newCache()

	cachedArchive := func(app string, targetDir string, archiveType archiveType) (string, error) {
		key := fmt.Sprintf("%s-%s", app, archiveType)
		return archiveCache.lookup(key, func() (string, error) {
			if apps.IsStorageReference(app) {
				return app, nil
			}

			return archive(app, targetDir, archiveType)
		})
	}

	cachedUpload := func(path string, description string, pType uploadType, dryRun bool) (string, error) {
		return uploadCache.lookup(path, func() (string, error) {
			return r.uploadArchive(ctx, storage.FileInfo{Name: path, Description: description}, pType, dryRun)
		})
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if !r.Project.DryRun {
		defer os.RemoveAll(tempDir)
	}
	if err != nil {
		return exitCode, err
	}

	for i, s := range r.Project.Suites {
		archiveType := zipArchive
		if len(s.Devices) > 0 {
			archiveType = ipaArchive
		}

		archivePath, err := cachedArchive(s.App, tempDir, archiveType)
		if err != nil {
			return exitCode, err
		}
		storageURL, err := cachedUpload(archivePath, s.AppDescription, appUpload, r.Project.DryRun)
		if err != nil {
			return exitCode, err
		}
		r.Project.Suites[i].App = storageURL

		archivePath, err = cachedArchive(s.TestApp, tempDir, archiveType)
		if err != nil {
			return exitCode, err
		}
		storageURL, err = cachedUpload(archivePath, s.TestAppDescription, testAppUpload, r.Project.DryRun)
		if err != nil {
			return exitCode, err
		}
		r.Project.Suites[i].TestApp = storageURL

		var otherApps []string
		for _, o := range s.OtherApps {
			archivePath, err = cachedArchive(o, tempDir, archiveType)
			if err != nil {
				return exitCode, err
			}
			storageURL, err = cachedUpload(archivePath, "", otherAppsUpload, r.Project.DryRun)
			if err != nil {
				return exitCode, err
			}
			otherApps = append(otherApps, storageURL)
		}
		r.Project.Suites[i].OtherApps = otherApps
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

func (r *XcuitestRunner) dryRun() {
	fmt.Println("\nThe following test suites would have run:")
	for _, s := range r.Project.Suites {
		fmt.Printf("  - %s\n", s.Name)
		for _, d := range s.Devices {
			fmt.Printf("    - on %s %s %s\n", d.Name, d.PlatformName, d.PlatformVersion)
		}
	}
	fmt.Println()
}

func (r *XcuitestRunner) runSuites(ctx context.Context) bool {
	jobOpts, results := r.createWorkerPool(ctx, r.Project.Sauce.Concurrency, r.Project.Sauce.Retries)
	defer close(results)

	suites := r.Project.Suites
	if r.Project.Sauce.LaunchOrder != "" {
		history, err := r.getHistory(ctx, r.Project.Sauce.LaunchOrder)
		if err != nil {
			log.Warn().Err(err).Msg(msg.RetrieveJobHistoryError)
		} else {
			suites = xcuitest.SortByHistory(suites, history)
		}
	}

	// Submit suites to work on.
	jobsCount := r.calculateJobsCount(suites)
	go func() {
		for _, s := range suites {
			for _, d := range enumerateDevices(s.Devices, s.Simulators) {
				log.Debug().Str("suite", s.Name).
					Str("deviceName", d.name).Str("deviceID", d.ID).
					Str("platformVersion", d.platformVersion).
					Msg("Starting job")
				r.startJob(jobOpts, s.App, s.TestApp, s.OtherApps, s, d)
			}
		}
	}()

	return r.collectResults(ctx, results, jobsCount)
}

func (r *XcuitestRunner) startJob(jobOpts chan<- job.StartOptions, appFileID, testAppFileID string, otherAppsIDs []string, s xcuitest.Suite, d deviceConfig) {
	jobOpts <- job.StartOptions{
		ConfigFilePath:   r.Project.ConfigFilePath,
		CLIFlags:         r.Project.CLIFlags,
		DisplayName:      s.Name,
		Timeout:          s.Timeout,
		App:              appFileID,
		TestApp:          testAppFileID,
		Suite:            testAppFileID,
		OtherApps:        otherAppsIDs,
		Framework:        "xcuitest",
		FrameworkVersion: "1.0.0-stable",
		PlatformName:     d.platformName,
		PlatformVersion:  d.platformVersion,
		DeviceName:       d.name,
		DeviceID:         d.ID,
		Name:             s.Name,
		Build:            r.Project.Sauce.Metadata.Build,
		Tags:             r.Project.Sauce.Metadata.Tags,
		Tunnel: job.TunnelOptions{
			Name:  r.Project.Sauce.Tunnel.Name,
			Owner: r.Project.Sauce.Tunnel.Owner,
		},
		Experiments:   r.Project.Sauce.Experiments,
		TestsToRun:    s.TestOptions.Class,
		TestsToSkip:   s.TestOptions.NotClass,
		Attempt:       0,
		Retries:       r.Project.Sauce.Retries,
		PassThreshold: s.PassThreshold,
		SmartRetry: job.SmartRetry{
			FailedOnly: s.SmartRetry.IsRetryFailedOnly(),
		},
		TestOptions: s.TestOptions.ToMap(),

		// RDC Specific flags
		RealDevice:        d.isRealDevice,
		DeviceHasCarrier:  d.hasCarrier,
		DeviceType:        d.deviceType,
		DevicePrivateOnly: d.privateOnly,

		// VMD specific settings
		Env:         s.Env,
		ARMRequired: d.armRequired,

		// Configure device settings
		RealDeviceKind: strings.ToLower(xcuitest.IOS),
		AppSettings: job.AppSettings{
			ResigningEnabled: s.AppSettings.ResigningEnabled,
			AudioCapture:     s.AppSettings.AudioCapture,
			Resigning: job.Resigning{
				ImageInjection:         s.AppSettings.Instrumentation.ImageInjection,
				GroupDirectory:         s.AppSettings.Instrumentation.GroupDirectory,
				SystemAlertsDelay:      s.AppSettings.Instrumentation.SysAlertsDelay,
				BiometricsInterception: s.AppSettings.Instrumentation.Biometrics,
				Vitals:                 s.AppSettings.Instrumentation.Vitals,
				NetworkCapture:         s.AppSettings.Instrumentation.NetworkCapture,
			},
		},
	}
}

func (r *XcuitestRunner) calculateJobsCount(suites []xcuitest.Suite) int {
	jobsCount := 0
	for _, s := range suites {
		jobsCount += len(enumerateDevices(s.Devices, s.Simulators))
	}
	return jobsCount
}
