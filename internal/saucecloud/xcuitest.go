package saucecloud

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

// XcuitestRunner represents the Sauce Labs cloud implementation for xcuitest.
type XcuitestRunner struct {
	CloudRunner
	Project xcuitest.Project
}

// cache represents a store that can be used to cache return values of functions.
type cache struct {
	store map[string]string
}

func newCache() cache {
	return cache{
		store: make(map[string]string),
	}
}

// lookup attempts to find the value for a key in the cache and returns if there's a hit, otherwise it executes and returns the closure fn
func (c cache) lookup(key string, fn func() (string, error)) (string, error) {
	var err error
	val, ok := c.store[key]
	if !ok {
		val, err = fn()
		if err == nil {
			c.store[key] = val
		}
	}
	return val, err
}

type archiveType string

var (
	ipaArchive archiveType = "ipa"
	zipArchive archiveType = "zip"
)

// RunProject runs the tests defined in xcuitest.Project.
func (r *XcuitestRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner, r.Project.DryRun); err != nil {
		return exitCode, err
	}

	archiveCache := newCache()
	uploadCache := newCache()

	cachedArchive := func(app string, archiveType archiveType) (string, error) {
		key := fmt.Sprintf("%s-%s", app, archiveType)
		return archiveCache.lookup(key, func() (string, error) {
			if apps.IsStorageReference(app) {
				return app, nil
			}

			return archive(app, archiveType)
		})
	}

	cachedUpload := func(path string, description string, pType uploadType, dryRun bool) (string, error) {
		return uploadCache.lookup(path, func() (string, error) {
			return r.uploadProject(path, description, pType, dryRun)
		})
	}

	for i, s := range r.Project.Suites {
		archiveType := zipArchive
		if len(s.Devices) > 0 {
			archiveType = ipaArchive
		}

		archivePath, err := cachedArchive(s.App, archiveType)
		if err != nil {
			return exitCode, err
		}
		storageUrl, err := cachedUpload(archivePath, s.AppDescription, appUpload, r.Project.DryRun)
		if err != nil {
			return exitCode, err
		}
		r.Project.Suites[i].App = storageUrl

		archivePath, err = cachedArchive(s.TestApp, archiveType)
		if err != nil {
			return exitCode, err
		}
		storageUrl, err = cachedUpload(archivePath, s.TestAppDescription, testAppUpload, r.Project.DryRun)
		if err != nil {
			return exitCode, err
		}
		r.Project.Suites[i].TestApp = storageUrl

		var otherApps []string
		for _, o := range s.OtherApps {
			archivePath, err = cachedArchive(o, archiveType)
			if err != nil {
				return exitCode, err
			}
			storageUrl, err = cachedUpload(archivePath, "", otherAppsUpload, r.Project.DryRun)
			if err != nil {
				return exitCode, err
			}
			otherApps = append(otherApps, storageUrl)
		}
		r.Project.Suites[i].OtherApps = otherApps
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

func (r *XcuitestRunner) runSuites() bool {
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
			suites = xcuitest.SortByHistory(suites, history)
		}
	}

	// Submit suites to work on.
	jobsCount := r.calculateJobsCount(suites)
	go func() {
		for _, s := range suites {
			for _, d := range s.Devices {
				log.Debug().Str("suite", s.Name).Str("deviceName", d.Name).Str("deviceID", d.ID).Str("platformVersion", d.PlatformVersion).Msg("Starting job")
				r.startJob(jobOpts, s.App, s.TestApp, s.OtherApps, s, d)
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

func (r *XcuitestRunner) startJob(jobOpts chan<- job.StartOptions, appFileID, testAppFileID string, otherAppsIDs []string, s xcuitest.Suite, d config.Device) {
	jobOpts <- job.StartOptions{
		ConfigFilePath:   r.Project.ConfigFilePath,
		CLIFlags:         r.Project.CLIFlags,
		DisplayName:      s.Name,
		Timeout:          s.Timeout,
		App:              appFileID,
		Suite:            testAppFileID,
		OtherApps:        otherAppsIDs,
		Framework:        "xcuitest",
		FrameworkVersion: "1.0.0-stable",
		PlatformName:     d.PlatformName,
		PlatformVersion:  d.PlatformVersion,
		DeviceName:       d.Name,
		DeviceID:         d.ID,
		Name:             s.Name,
		Build:            r.Project.Sauce.Metadata.Build,
		Tags:             r.Project.Sauce.Metadata.Tags,
		Tunnel: job.TunnelOptions{
			ID:     r.Project.Sauce.Tunnel.Name,
			Parent: r.Project.Sauce.Tunnel.Owner,
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

		// RDC Specific flags
		RealDevice:        true,
		DeviceHasCarrier:  d.Options.CarrierConnectivity,
		DeviceType:        d.Options.DeviceType,
		DevicePrivateOnly: d.Options.Private,

		// Overwrite device settings
		RealDeviceKind: strings.ToLower(xcuitest.IOS),
		AppSettings: job.AppSettings{
			AudioCapture: s.AppSettings.AudioCapture,
			Instrumentation: job.Instrumentation{
				NetworkCapture: s.AppSettings.Instrumentation.NetworkCapture,
			},
		},
	}
}

func (r *XcuitestRunner) calculateJobsCount(suites []xcuitest.Suite) int {
	jobsCount := 0
	for _, s := range suites {
		jobsCount += len(s.Devices)
		jobsCount += len(s.Simulators)
	}
	return jobsCount
}

func archive(src string, archiveType archiveType) (string, error) {
	switch archiveType {
	case ipaArchive:
		return archiveAppToIpa(src)
	case zipArchive:
		return archiveAppToZip(src)
	}
	return "", fmt.Errorf("unknown archive type: %s", archiveType)
}

func archiveAppToZip(appPath string) (string, error) {
	if strings.HasSuffix(appPath, ".zip") {
		return appPath, nil
	}

	log.Info().Msgf("Archiving %s to .zip", path.Base(appPath))
	fileName := fmt.Sprintf("%s-*.zip", strings.TrimSuffix(path.Base(appPath), ".app"))
	tmpFile, err := os.CreateTemp(os.TempDir(), fileName)
	if err != nil {
		return "", err
	}
	arch, _ := zip.New(tmpFile, sauceignore.NewMatcher([]sauceignore.Pattern{}))
	defer arch.Close()
	_, _, err = arch.Add(appPath, "")
	if err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

// archiveAppToIpa generates a valid IPA file from a .app folder.
func archiveAppToIpa(appPath string) (string, error) {
	if strings.HasSuffix(appPath, ".ipa") {
		return appPath, nil
	}

	log.Info().Msgf("Archiving %s to .ipa", path.Base(appPath))
	fileName := fmt.Sprintf("%s-*.ipa", strings.TrimSuffix(path.Base(appPath), ".app"))
	tmpFile, err := os.CreateTemp(os.TempDir(), fileName)
	if err != nil {
		return "", err
	}
	arch, _ := zip.New(tmpFile, sauceignore.NewMatcher([]sauceignore.Pattern{}))
	defer arch.Close()
	_, _, err = arch.Add(appPath, "Payload/")
	if err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}
