package saucecloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	ptable "github.com/jedib0t/go-pretty/v6/table"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader       storage.ProjectUploader
	JobStarter            job.Starter
	JobReader             job.Reader
	RDCJobReader          job.Reader
	JobWriter             job.Writer
	JobStopper            job.Stopper
	CCYReader             concurrency.Reader
	TunnelService         tunnel.Service
	Region                region.Region
	MetadataService       framework.MetadataService
	ShowConsoleLog        bool
	ArtifactDownloader    download.ArtifactDownloader
	RDCArtifactDownloader download.ArtifactDownloader
	Framework             framework.Framework

	Reporters []report.Reporter

	Async bool

	interrupted bool
}

type result struct {
	name      string
	browser   string
	job       job.Job
	skipped   bool
	err       error
	duration  time.Duration
	startTime time.Time
	endTime   time.Time
	attempts  int
	retries   int
}

// ConsoleLogAsset represents job asset log file name.
const ConsoleLogAsset = "console.log"

func (r *CloudRunner) createWorkerPool(ccy int, maxRetries int) (chan job.StartOptions, chan result, error) {
	jobOpts := make(chan job.StartOptions, maxRetries+1)
	results := make(chan result, ccy)

	log.Info().Int("concurrency", ccy).Msg("Launching workers.")
	for i := 0; i < ccy; i++ {
		go r.runJobs(jobOpts, results)
	}

	return jobOpts, results, nil
}

func (r *CloudRunner) collectResults(artifactCfg config.ArtifactDownload, results chan result, expected int) bool {
	// TODO find a better way to get the expected
	completed := 0
	inProgress := expected
	passed := true

	junitRequired := report.IsArtifactRequired(r.Reporters, report.JUnitArtifact)

	done := make(chan interface{})
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				log.Info().Msgf("Suites in progress: %d", inProgress)
			}
		}
	}()
	for i := 0; i < expected; i++ {
		res := <-results
		// in case one of test suites not passed
		if !res.job.Passed {
			passed = false
		}
		completed++
		inProgress--

		if !res.skipped {
			platform := res.job.BaseConfig.PlatformName
			if res.job.BaseConfig.PlatformVersion != "" {
				platform = fmt.Sprintf("%s %s", platform, res.job.BaseConfig.PlatformVersion)
			}

			browser := res.browser
			// browser is empty for mobile tests
			if browser != "" {
				browser = fmt.Sprintf("%s %s", browser, res.job.BrowserShortVersion)
			}

			var artifacts []report.Artifact

			if junitRequired {
				jb, err := r.getAsset(res.job.ID, "junit.xml", res.job.IsRDC)
				artifacts = append(artifacts, report.Artifact{
					AssetType: report.JUnitArtifact,
					Body:      jb,
					Error:     err,
				})
			}

			url := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), res.job.ID)
			tr := report.TestResult{
				Name:       res.name,
				Duration:   res.duration,
				StartTime:  res.startTime,
				EndTime:    res.endTime,
				Status:     res.job.TotalStatus(),
				Browser:    browser,
				Platform:   platform,
				DeviceName: res.job.BaseConfig.DeviceName,
				URL:        url,
				Artifacts:  artifacts,
				Origin:     "sauce",
				Attempts:   res.attempts,
			}

			for _, rep := range r.Reporters {
				rep.Add(tr)
			}
		}

		if download.ShouldDownloadArtifact(res.job.ID, res.job.Passed, res.job.TimedOut, r.Async, artifactCfg) {
			if res.job.IsRDC {
				r.RDCArtifactDownloader.DownloadArtifact(res.job.ID)
			} else {
				r.ArtifactDownloader.DownloadArtifact(res.job.ID)
			}
		}

		// Since we don't know much about the state of the job in async mode, we'll just
		r.logSuite(res)
	}
	close(done)

	for _, rep := range r.Reporters {
		rep.Render()
	}

	return passed
}

func (r *CloudRunner) getAsset(jobID string, name string, rdc bool) ([]byte, error) {
	if rdc {
		return r.RDCJobReader.GetJobAssetFileContent(context.Background(), jobID, name)
	}

	return r.JobReader.GetJobAssetFileContent(context.Background(), jobID, name)
}

func (r *CloudRunner) runJob(opts job.StartOptions) (j job.Job, skipped bool, err error) {
	log.Info().Str("suite", opts.DisplayName).Str("region", r.Region.String()).Msg("Starting suite.")

	id, isRDC, err := r.JobStarter.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{}, false, err
	}

	if !isRDC {
		r.uploadSauceConfig(id, opts.ConfigFilePath)
		r.uploadCLIFlags(id, opts.CLIFlags)
	}
	// os.Interrupt can arrive before the signal.Notify() is registered. In that case,
	// if a soft exit is requested during startContainer phase, it gently exits.
	if r.interrupted {
		return job.Job{}, true, nil
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), id)
	l := log.Info().Str("url", jobDetailsPage).Str("suite", opts.DisplayName).Str("platform", opts.PlatformName)

	// FIXME framework specifics shouldn't be handled in a generic package. Could simply do an empty string check instead.
	if opts.Framework == "espresso" {
		l.Str("deviceName", opts.DeviceName).Str("platformVersion", opts.PlatformVersion).Str("deviceId", opts.DeviceID)
		if isRDC {
			l.Bool("private", opts.DevicePrivateOnly)
		}
	} else {
		l.Str("browser", opts.BrowserName)
	}

	l.Msg("Suite started.")

	// Async mode. Mark the job as started without waiting for the result.
	if r.Async {
		return job.Job{ID: id, IsRDC: isRDC, Status: job.StateInProgress}, false, nil
	}

	// High interval poll to not oversaturate the job reader with requests
	if !isRDC {
		sigChan := r.registerInterruptOnSignal(id, opts.DisplayName)
		defer unregisterSignalCapture(sigChan)

		j, err = r.JobReader.PollJob(context.Background(), id, 15*time.Second, opts.Timeout)
	} else {
		j, err = r.RDCJobReader.PollJob(context.Background(), id, 15*time.Second, opts.Timeout)
	}

	if err != nil {
		return job.Job{}, false, fmt.Errorf("failed to retrieve job status for suite %s: %s", opts.DisplayName, err.Error())
	}

	// Enrich RDC data
	if isRDC {
		enrichRDCReport(&j, opts)
	}

	// Check timeout
	if j.TimedOut {
		color.Red("Suite '%s' has reached %ds timeout", opts.DisplayName, opts.Timeout)
		if !isRDC {
			j, err = r.JobStopper.StopJob(context.Background(), id)
			if err != nil {
				color.HiRedString("Failed to stop suite '%s': %v", opts.DisplayName, err)
			}
		}
		j.Passed = false
		j.TimedOut = true
		return j, false, fmt.Errorf("suite '%s' has reached timeout", opts.DisplayName)
	}

	if !j.Passed {
		// We may need to differentiate when a job has crashed vs. when there is errors.
		return j, false, fmt.Errorf("suite '%s' has test failures", opts.DisplayName)
	}

	return j, false, nil
}

// enrichRDCReport added the fields from the opts as the API does not provides it.
func enrichRDCReport(j *job.Job, opts job.StartOptions) {
	switch opts.Framework {
	case "espresso":
		j.BaseConfig.PlatformName = espresso.Android
	}

	if opts.DeviceID != "" {
		j.BaseConfig.DeviceName = opts.DeviceID
	} else {
		j.BaseConfig.DeviceName = opts.DeviceName
		j.BaseConfig.PlatformVersion = opts.PlatformVersion
	}
}

func (r *CloudRunner) runJobs(jobOpts chan job.StartOptions, results chan<- result) {
	for opts := range jobOpts {
		start := time.Now()

		if r.interrupted {
			results <- result{
				name:     opts.DisplayName,
				browser:  opts.BrowserName,
				skipped:  true,
				err:      nil,
				attempts: opts.Attempt + 1,
				retries:  opts.Retries,
			}
			continue
		}

		if opts.Attempt == 0 {
			opts.StartTime = start
		}

		jobData, skipped, err := r.runJob(opts)

		if opts.Attempt < opts.Retries && !jobData.Passed {
			log.Warn().Err(err).Msg("Suite errored.")
			opts.Attempt++
			jobOpts <- opts
			log.Info().Str("suite", opts.DisplayName).Str("attempt", fmt.Sprintf("%d of %d", opts.Attempt+1, opts.Retries+1)).Msg("Retrying suite.")
			continue
		}

		results <- result{
			name:      opts.DisplayName,
			browser:   opts.BrowserName,
			job:       jobData,
			skipped:   skipped,
			err:       err,
			startTime: opts.StartTime,
			endTime:   time.Now(),
			duration:  time.Since(start),
			attempts:  opts.Attempt + 1,
			retries:   opts.Retries,
		}
	}
}

func (r CloudRunner) archiveAndUpload(project interface{}, folder string, sauceignoreFile string) (string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveProject(project, tempDir, folder, sauceignoreFile)
	if err != nil {
		return "", err
	}

	return r.uploadProject(zipName, projectUpload)
}

func (r *CloudRunner) archiveProject(project interface{}, tempDir string, projectFolder string, sauceignoreFile string) (string, error) {
	start := time.Now()

	matcher, err := sauceignore.NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		return "", err
	}

	zipName := filepath.Join(tempDir, "app.zip")
	z, err := zip.NewFileWriter(zipName, matcher)
	if err != nil {
		return "", err
	}
	defer z.Close()

	rcPath := filepath.Join(tempDir, "sauce-runner.json")
	if err := jsonio.WriteFile(rcPath, project); err != nil {
		return "", err
	}

	folderContent, err := os.ReadDir(projectFolder)
	if err != nil {
		return "", err
	}

	for _, child := range folderContent {
		log.Debug().Str("name", child.Name()).Msg("Adding to archive")
		if err := z.Add(filepath.Join(projectFolder, child.Name()), ""); err != nil {
			return "", err
		}
	}
	log.Debug().Str("name", rcPath).Msg("Adding to archive")
	if err := z.Add(rcPath, ""); err != nil {
		return "", err
	}

	err = z.Close()
	if err != nil {
		return "", err
	}

	f, err := os.Stat(zipName)
	if err != nil {
		return "", err
	}

	log.Info().Dur("durationMs", time.Since(start)).Int64("size", f.Size()).Msg("Project archived.")

	return zipName, nil
}

type uploadType string

var (
	testAppUpload   uploadType = "test application"
	appUpload       uploadType = "application"
	projectUpload   uploadType = "project"
	otherAppsUpload uploadType = "other applications"
)

func (r *CloudRunner) uploadProjects(filename []string, pType uploadType) ([]string, error) {
	var IDs []string
	for _, f := range filename {
		ID, err := r.uploadProject(f, pType)
		if err != nil {
			return []string{}, err
		}
		IDs = append(IDs, ID)
	}

	return IDs, nil
}

func (r *CloudRunner) uploadProject(filename string, pType uploadType) (string, error) {
	if apps.IsStorageReference(filename) {
		return apps.StandardizeReferenceLink(filename), nil
	}

	log.Info().Msgf("Checking if %s has already been uploaded previously", filename)
	if storageID, _ := r.checkIfFileAlreadyUploaded(filename); storageID != "" {
		log.Info().Msgf("Skipping upload, using storage:%s", storageID)
		return fmt.Sprintf("storage:%s", storageID), nil
	}

	filename, err := filepath.Abs(filename)
	if err != nil {
		return "", nil
	}
	progress.Show("Uploading %s %s", pType, filename)

	start := time.Now()
	resp, err := r.ProjectUploader.Upload(filename)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Dur("durationMs", time.Since(start)).Str("storageId", resp.ID).Msgf("%s uploaded.", strings.Title(string(pType)))
	return fmt.Sprintf("storage:%s", resp.ID), nil
}

func (r *CloudRunner) checkIfFileAlreadyUploaded(fileName string) (storageID string, err error) {
	resp, err := r.ProjectUploader.Find(fileName)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// logSuite display the result of a suite
func (r *CloudRunner) logSuite(res result) {
	// Job isn't done, hence nothing more to log about it.
	if !job.Done(res.job.Status) {
		return
	}

	if res.skipped {
		log.Error().Err(res.err).Str("suite", res.name).Msg("Suite skipped.")
		return
	}
	if res.job.ID == "" {
		log.Error().Err(res.err).Str("suite", res.name).Msg("Failed to start suite.")
		return
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), res.job.ID)
	msg := "Suite finished."
	if res.job.Passed {
		log.Info().Str("suite", res.name).Bool("passed", res.job.Passed).Str("url", jobDetailsPage).
			Msg(msg)
	} else {
		l := log.Error().Str("suite", res.name).Bool("passed", res.job.Passed).Str("url", jobDetailsPage)
		if res.job.TotalStatus() == job.StateError {
			l.Str("error", res.job.Error)
			msg = "Suite finished with error."
		}
		l.Msg(msg)
	}
	r.logSuiteConsole(res)
}

// logSuiteError display the console output when tests from a suite are failing
func (r *CloudRunner) logSuiteConsole(res result) {
	// To avoid clutter, we don't show the console on job passes.
	if res.job.Passed && !r.ShowConsoleLog {
		return
	}

	jr := r.JobReader
	if res.job.IsRDC {
		jr = r.RDCJobReader
	}

	var assetContent []byte
	var err error

	// Display log only when at least it has started
	if assetContent, err = jr.GetJobAssetFileContent(context.Background(), res.job.ID, ConsoleLogAsset); err == nil {
		log.Info().Str("suite", res.name).Msgf("console.log output: \n%s", assetContent)
		return
	}

	// Some frameworks produce a junit.xml instead, check for that file if there's no console.log
	assetContent, err = jr.GetJobAssetFileContent(context.Background(), res.job.ID, "junit.xml")
	if err != nil {
		log.Warn().Err(err).Str("suite", res.name).Msg("Failed to retrieve the console output.")
		return
	}

	var testsuites junit.TestSuites
	if testsuites, err = junit.Parse(assetContent); err != nil {
		log.Warn().Str("suite", res.name).Msg("Failed to parse junit")
		return
	}

	// Print summary of failures from junit.xml
	headerColor := color.New(color.FgRed).Add(color.Bold).Add(color.Underline)
	if !res.job.Passed {
		headerColor.Print("\nErrors:\n\n")
	}
	bodyColor := color.New(color.FgHiRed)
	errCount := 1
	failCount := 1
	for _, ts := range testsuites.TestSuites {
		for _, tc := range ts.TestCases {
			if tc.Error != "" {
				fmt.Printf("\n\t%d) %s.%s\n\n", errCount, tc.ClassName, tc.Name)
				headerColor.Println("\tError was:")
				bodyColor.Printf("\t%s\n", tc.Error)
				errCount++
			} else if tc.Failure != "" {
				fmt.Printf("\n\t%d) %s.%s\n\n", failCount, tc.ClassName, tc.Name)
				headerColor.Println("\tFailure was:")
				bodyColor.Printf("\t%s\n", tc.Failure)
				failCount++
			}
		}
	}

	fmt.Println()
	t := ptable.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(ptable.Row{fmt.Sprintf("%s testsuite", r.Framework.Name), "tests", "pass", "fail", "error"})
	for _, ts := range testsuites.TestSuites {
		passed := ts.Tests - ts.Errors - ts.Failures
		t.AppendRow(ptable.Row{ts.Package, ts.Tests, passed, ts.Failures, ts.Errors})
	}
	t.Render()
	fmt.Println()
}

func (r *CloudRunner) validateTunnel(name, owner string) error {
	if name == "" {
		return nil
	}

	// This wait value is deliberately not configurable.
	wait := 30 * time.Second
	log.Info().Str("timeout", wait.String()).Msg("Performing tunnel readiness check...")
	if err := r.TunnelService.IsTunnelRunning(context.Background(), name, owner, wait); err != nil {
		return err
	}

	log.Info().Msg("Tunnel is ready!")
	return nil
}

func (r *CloudRunner) dryRun(project interface{}, folder string, sauceIgnoreFile string, suiteNames string) error {
	log.Warn().Msg("Running tests in dry run mode.")
	tmpDir, err := os.MkdirTemp("./", "sauce-app-payload-*")
	if err != nil {
		return err
	}
	log.Info().Msgf("The following test suites would have run: [%s].", suiteNames)
	zipName, err := r.archiveProject(project, tmpDir, folder, sauceIgnoreFile)
	if err != nil {
		return err
	}

	log.Info().Msgf("Saving bundled project to %s.", zipName)
	return nil
}

// stopSuiteExecution stops the current execution on Sauce Cloud
func (r *CloudRunner) stopSuiteExecution(jobID string, suiteName string) {
	_, err := r.JobStopper.StopJob(context.Background(), jobID)
	if err != nil {
		log.Warn().Err(err).Str("suite", suiteName).Msg("Unable to stop suite.")
	}
}

// registerInterruptOnSignal stops execution on Sauce Cloud when a SIGINT is captured.
func (r *CloudRunner) registerInterruptOnSignal(jobID, suiteName string) chan os.Signal {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, jobID, suiteName string) {
		sig := <-c
		if sig == nil {
			return
		}
		log.Info().Str("suite", suiteName).Msg("Stopping suite")
		r.stopSuiteExecution(jobID, suiteName)
	}(sigChan, jobID, suiteName)
	return sigChan
}

// registerSkipSuitesOnSignal prevent new suites from being executed when a SIGINT is captured.
func (r *CloudRunner) registerSkipSuitesOnSignal() chan os.Signal {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, cr *CloudRunner) {
		for {
			sig := <-c
			if sig == nil {
				return
			}
			if cr.interrupted {
				os.Exit(1)
			}
			log.Info().Msg("Ctrl-C captured. Ctrl-C again to exit now.")
			cr.interrupted = true
		}
	}(sigChan, r)
	return sigChan
}

// unregisterSignalCapture remove the signal hook associated to the chan c.
func unregisterSignalCapture(c chan os.Signal) {
	signal.Stop(c)
	close(c)
}

// uploadSauceConfig adds job configuration as an asset.
func (r *CloudRunner) uploadSauceConfig(jobID string, cfgFile string) {
	// A config file is optional.
	if cfgFile == "" {
		return
	}

	f, err := os.Open(cfgFile)
	if err != nil {
		log.Warn().Msgf("failed to open configuration: %v", err)
		return
	}
	content, err := io.ReadAll(f)
	if err != nil {
		log.Warn().Msgf("failed to read configuration: %v", err)
		return
	}
	if err := r.JobWriter.UploadAsset(jobID, filepath.Base(cfgFile), "text/plain", content); err != nil {
		log.Warn().Msgf("failed to attach configuration: %v", err)
	}
}

// uploadCLIFlags adds commandline parameters as an asset.
func (r *CloudRunner) uploadCLIFlags(jobID string, content interface{}) {
	encoded, err := json.Marshal(content)
	if err != nil {
		log.Warn().Msgf("Failed to encode CLI flags: %v", err)
		return
	}
	if err := r.JobWriter.UploadAsset(jobID, "flags.json", "text/plain", encoded); err != nil {
		log.Warn().Msgf("Failed to report CLI flags: %v", err)
	}
}

// checkVersion checks if the requested version is available on Cloud.
func (r *CloudRunner) checkVersionAvailability(frameworkName string, frameworkVersion string) error {
	metadata, err := r.MetadataService.Search(context.Background(), framework.SearchOptions{
		Name:             frameworkName,
		FrameworkVersion: frameworkVersion,
	})
	if err != nil && isUnsupportedVersion(err) {
		color.Red(fmt.Sprintf("\nVersion %s for %s is not available !\n\n", frameworkVersion, frameworkName))
		r.logAvailableVersions(frameworkName)
		return errors.New("unsupported framework version")
	}
	if err != nil {
		return errors.New(fmt.Sprintf("unable to check framework version availability: %v", err))
	}
	if metadata.Deprecated {
		color.Red(fmt.Sprintf("\nVersion %s for %s is deprecated and will be removed during our next framework release cycle !\n\n", frameworkVersion, frameworkName))
		fmt.Printf("You should update your version of %s to a more recent one.\n", frameworkName)
		r.logAvailableVersions(frameworkName)
	}
	return nil
}

// logAvailableVersions displays the available cloud version for the framework.
func (r *CloudRunner) logAvailableVersions(frameworkName string) {
	versions, err := r.MetadataService.Versions(context.Background(), frameworkName)
	if err != nil {
		return
	}
	fmt.Printf("Available versions of %s are:\n", frameworkName)
	for _, v := range versions {
		if !v.Deprecated {
			fmt.Printf(" - %s\n", v.FrameworkVersion)
		}
	}
	println()
}

// isUnsupportedVersion returns true if the error is an unsupported version.
func isUnsupportedVersion(err error) bool {
	return strings.Contains(err.Error(), "Bad Request: unsupported version")
}
