package saucecloud

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
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

	if r.Project.DryRun {
		r.dryRun()
		return 0, nil
	}

	err := archiveAppsToIpaIfRequired(&r.Project)
	if err != nil {
		return exitCode, err
	}

	r.Project.Xcuitest.App, err = r.uploadProject(r.Project.Xcuitest.App, appUpload, r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}

	r.Project.Xcuitest.OtherApps, err = r.uploadProjects(r.Project.Xcuitest.OtherApps, otherAppsUpload, r.Project.DryRun)
	if err != nil {
		return exitCode, err
	}
	for i, s := range r.Project.Suites {
		r.Project.Suites[i].TestApp, err = r.uploadProject(s.TestApp, testAppUpload, r.Project.DryRun)
		if err != nil {
			return exitCode, err
		}
	}

	passed := r.runSuites()
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *XcuitestRunner) dryRun() {
	log.Warn().Msg("Running tests in dry run mode.")
	for _, s := range r.Project.Suites {
		for _, d := range s.Devices {
			log.Info().Msgf("The [%s] suite would run on %s %s %s", s.Name, d.Name, d.PlatformName, d.PlatformVersion)
		}
	}
}

func (r *XcuitestRunner) runSuites() bool {
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
			for _, d := range s.Devices {
				log.Debug().Str("suite", s.Name).Str("deviceName", d.Name).Str("deviceID", d.ID).Str("platformVersion", d.PlatformVersion).Msg("Starting job")
				r.startJob(jobOpts, r.Project.Xcuitest.App, s.TestApp, r.Project.Xcuitest.OtherApps, s, d)
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
		Experiments: r.Project.Sauce.Experiments,
		TestsToRun:  s.TestOptions.Class,
		TestsToSkip: s.TestOptions.NotClass,
		Attempt:     0,
		Retries:     r.Project.Sauce.Retries,

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
func archiveAppsToIpaIfRequired(project *xcuitest.Project) (err error) {
	appPath := project.Xcuitest.App
	if !strings.HasSuffix(appPath, ".ipa") {
		project.Xcuitest.App, err = archiveAppToIpa(appPath)
		if err != nil {
			log.Error().Msgf("Unable to archive %s to ipa: %v", appPath, err)
			err = fmt.Errorf("unable to archive %s", appPath)
			return
		}
	}
	for i, s := range project.Suites {
		if !strings.HasSuffix(s.TestApp, ".ipa") {
			project.Suites[i].TestApp, err = archiveAppToIpa(s.TestApp)
			if err != nil {
				log.Error().Msgf("Unable to archive %s to ipa: %v", s.TestApp, err)
				err = fmt.Errorf("unable to archive %s", s.TestApp)
				return
			}
		}
	}
	return
}

// archiveAppToIpa generates a valid IPA file from a .app folder.
func archiveAppToIpa(appPath string) (string, error) {
	log.Info().Msgf("Archiving %s to .ipa", path.Base(appPath))
	fileName := fmt.Sprintf("%s-*.ipa", strings.TrimSuffix(path.Base(appPath), ".app"))
	tmpFile, err := ioutil.TempFile(os.TempDir(), fileName)
	if err != nil {
		return "", err
	}
	arch, _ := zip.New(tmpFile, sauceignore.NewMatcher([]sauceignore.Pattern{}))
	defer arch.Close()
	_, err = arch.Add(appPath, "Payload/")
	if err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}
