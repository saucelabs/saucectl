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

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.ID); err != nil {
		return exitCode, err
	}

	if r.Project.DryRun {
		r.dryRun()
		return 0, nil
	}

	appPath, testAppPath, err := archiveAppsToIpaIfRequired(r.Project.Xcuitest.App, r.Project.Xcuitest.TestApp)
	if err != nil {
		return exitCode, err
	}

	appFileID, err := r.uploadProject(appPath, appUpload)
	if err != nil {
		return exitCode, err
	}

	testAppFileID, err := r.uploadProject(testAppPath, testAppUpload)
	if err != nil {
		return exitCode, err
	}

	otherAppsIDs, err := r.uploadProjects(r.Project.Xcuitest.OtherApps, otherAppsUpload)
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(appFileID, testAppFileID, otherAppsIDs)
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

func (r *XcuitestRunner) runSuites(appFileID, testAppFileID string, otherAppsIDs []string) bool {
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
				log.Debug().Str("suite", s.Name).Str("deviceName", d.Name).Str("deviceID", d.ID).Str("platformVersion", d.PlatformVersion).Msg("Starting job")
				r.startJob(jobOpts, appFileID, testAppFileID, otherAppsIDs, s, d)
			}
		}
		close(jobOpts)
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, jobsCount)
}

func (r *XcuitestRunner) startJob(jobOpts chan<- job.StartOptions, appFileID, testAppFileID string, otherAppsIDs []string, s xcuitest.Suite, d config.Device) {
	for i, ID := range otherAppsIDs {
		otherAppsIDs[i] = fmt.Sprintf("storage:%s", ID)
	}
	jobOpts <- job.StartOptions{
		ConfigFilePath:   r.Project.ConfigFilePath,
		CommandLine:      r.Project.CommandLine,
		DisplayName:      s.Name,
		App:              fmt.Sprintf("storage:%s", appFileID),
		Suite:            fmt.Sprintf("storage:%s", testAppFileID),
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
			ID:     r.Project.Sauce.Tunnel.ID,
			Parent: r.Project.Sauce.Tunnel.Parent,
		},
		Experiments: r.Project.Sauce.Experiments,
		TestsToRun:  s.TestOptions.Class,

		// RDC Specific flags
		RealDevice:        true,
		DeviceHasCarrier:  d.Options.CarrierConnectivity,
		DeviceType:        d.Options.DeviceType,
		DevicePrivateOnly: d.Options.Private,
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
func archiveAppsToIpaIfRequired(appPath, testAppPath string) (archivedAppPath string, archivedTestAppPath string, archivedErr error) {
	archivedAppPath = appPath
	archivedTestAppPath = testAppPath
	var err error
	if !strings.HasSuffix(appPath, ".ipa") {
		archivedAppPath, err = archiveAppToIpa(appPath)
		if err != nil {
			log.Error().Msgf("Unable to archive %s to ipa: %v", appPath, err)
			archivedErr = fmt.Errorf("unable to archive %s", appPath)
			return
		}
	}
	if !strings.HasSuffix(testAppPath, ".ipa") {
		archivedTestAppPath, err = archiveAppToIpa(testAppPath)
		if err != nil {
			log.Error().Msgf("Unable to archive %s to ipa: %v", testAppPath, err)
			fmt.Errorf("unable to archive %s", testAppPath)
			return
		}
	}
	return archivedAppPath, archivedTestAppPath, nil
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
	err = arch.Add(appPath, "Payload/")
	if err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}
