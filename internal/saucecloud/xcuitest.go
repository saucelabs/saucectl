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

// RunProject runs the tests defined in xcuitest.Project.
func (r *XcuitestRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner, r.Project.DryRun); err != nil {
		return exitCode, err
	}

	// err := archiveAppsToIpaIfRequired(&r.Project)
	// if err != nil {
	// 	return exitCode, err
	// }

	// r.Project.Xcuitest.App, err = r.uploadProject(r.Project.Xcuitest.App, r.Project.Xcuitest.AppDescription, appUpload, r.Project.DryRun)
	// if err != nil {
	// 	return exitCode, err
	// }

	// r.Project.Xcuitest.OtherApps, err = r.uploadProjects(r.Project.Xcuitest.OtherApps, otherAppsUpload, r.Project.DryRun)
	// if err != nil {
	// 	return exitCode, err
	// }

	cache := map[string]string{}
	for i, s := range r.Project.Suites {
		var val string
		var ok bool
		var err error
		val, ok = cache[s.TestApp]
		if !ok {
			val, err = r.uploadProject(s.TestApp, s.TestAppDescription, testAppUpload, r.Project.DryRun)
			if err != nil {
				return exitCode, err
			}
			cache[s.TestApp] = val
		}
		r.Project.Suites[i].TestApp = val

		val, ok = cache[s.App]
		if !ok {
			val, err = r.uploadProject(s.App, s.AppDescription, appUpload, r.Project.DryRun)
			if err != nil {
				return exitCode, err
			}
			cache[s.App] = val
		}
		r.Project.Suites[i].App = val

		// for _, o := range s.OtherApps {
		// 	val, ok = cache[o]
		// 	if !ok {
		// 		val, err = r.uploadProject(o, s.AppDescription, appUpload, r.Project.DryRun)
		// 		if err != nil {
		// 			return exitCode, err
		// 		}
		// 		cache[s.App] = val
		// 	}
		// }
		// 	if val, ok := cache[s.TestApp]; ok {
		// 		r.Project.Suites[i].TestApp = val
		// 		continue
		// 	}

		// 	testAppURL, err := r.uploadProject(s.TestApp, s.TestAppDescription, testAppUpload, r.Project.DryRun)
		// 	if err != nil {
		// 		return exitCode, err
		// 	}
		// 	r.Project.Suites[i].TestApp = testAppURL
		// 	cache[s.TestApp] = testAppURL
	}

	fmt.Printf("%+v", r.Project)
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
				// r.startJob(jobOpts, r.Project.Xcuitest.App, s.TestApp, r.Project.Xcuitest.OtherApps, s, d)
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
	}
	return jobsCount
}

// archiveAppsToIpaIfRequired checks if apps are a .ipa package. Otherwise, it generates one.
func archiveAppsToIpaIfRequired(project *xcuitest.Project) error {
	var err error
	cache := map[string]string{}
	// project.Xcuitest.App, err = archiveAppToIpaIfRequired(project.Xcuitest.App, cache)
	// if err != nil {
	// 	return err
	// }

	for i, s := range project.Suites {
		project.Suites[i].App, err = archiveAppToIpaIfRequired(s.App, cache)
		if err != nil {
			return err
		}
		project.Suites[i].TestApp, err = archiveAppToIpaIfRequired(s.TestApp, cache)
		if err != nil {
			return err
		}
	}
	return nil
}

func archiveAppToIpaIfRequired(appPath string, cache map[string]string) (string, error) {
	if apps.IsStorageReference(appPath) {
		return appPath, nil
	}

	if strings.HasSuffix(appPath, ".ipa") {
		return appPath, nil
	}

	if cachedApp, ok := cache[appPath]; ok {
		return cachedApp, nil
	}

	archivedApp, err := archiveAppToIpa(appPath)
	if err != nil {
		log.Error().Msgf("Unable to archive %s to ipa: %v", appPath, err)
		err = fmt.Errorf("unable to archive %s", appPath)
		return "", err
	}

	cache[appPath] = archivedApp
	return archivedApp, nil
}

// archiveAppToIpa generates a valid IPA file from a .app folder.
func archiveAppToIpa(appPath string) (string, error) {
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
