package saucecloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/node"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/fatih/color"
	ptable "github.com/jedib0t/go-pretty/v6/table"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader        storage.ProjectUploader
	JobService             job.Service
	CCYReader              concurrency.Reader
	TunnelService          tunnel.Service
	Region                 region.Region
	MetadataService        framework.MetadataService
	ShowConsoleLog         bool
	Framework              framework.Framework
	MetadataSearchStrategy framework.MetadataSearchStrategy

	Reporters []report.Reporter

	Async    bool
	FailFast bool

	NPMDependencies []string

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

// BaseFilepathLength represents the path length where project will be unpacked.
// Example: "D:\sauce-playwright-runner\1.12.0\bundle\__project__\"
const BaseFilepathLength = 53

// MaxFilepathLength represents the maximum path length acceptable.
const MaxFilepathLength = 255

// ArchiveFileCountSoftLimit is the threshold count of files added to the archive
// before a warning is printed.
// The value here (2^15) is somewhat arbitrary. In testing, ~32K files in the archive
// resulted in about 30s for download and extraction.
const ArchiveFileCountSoftLimit = 32768

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
	jsonResultRequired := report.IsArtifactRequired(r.Reporters, report.JSONArtifact)

	done := make(chan interface{})
	go func(r *CloudRunner) {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if !r.interrupted {
					log.Info().Msgf("Suites in progress: %d", inProgress)
				}
			}
		}
	}(r)
	for i := 0; i < expected; i++ {
		res := <-results
		// in case one of test suites not passed
		// ignore jobs that are still in progress (i.e. async execution or client timeout)
		// since their status is unknown
		if job.Done(res.job.Status) && !res.job.Passed {
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
				jb, err := r.JobService.GetJobAssetFileContent(
					context.Background(),
					res.job.ID,
					"junit.xml",
					res.job.IsRDC)
				artifacts = append(artifacts, report.Artifact{
					AssetType: report.JUnitArtifact,
					Body:      jb,
					Error:     err,
				})
			}

			var url string
			if res.job.ID != "" {
				url = fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), res.job.ID)
			}
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
				RDC:        res.job.IsRDC,
				TimedOut:   res.job.TimedOut,
			}

			var files []string
			if config.ShouldDownloadArtifact(res.job.ID, res.job.Passed, res.job.TimedOut, r.Async, artifactCfg) {
				files = r.JobService.DownloadArtifact(res.job.ID, res.name, res.job.IsRDC)
			}
			if jsonResultRequired {
				for _, f := range files {
					artifacts = append(artifacts, report.Artifact{
						FilePath: f,
					})
				}
			}

			for _, rep := range r.Reporters {
				rep.Add(tr)
			}
		}
		// Since we don't know much about the state of the job in async mode, we'll just
		r.logSuite(res)
	}
	close(done)

	if !r.interrupted {
		for _, rep := range r.Reporters {
			rep.Render()
		}
	}

	return passed
}

func (r *CloudRunner) runJob(opts job.StartOptions) (j job.Job, skipped bool, err error) {
	log.Info().Str("suite", opts.DisplayName).Str("region", r.Region.String()).Msg("Starting suite.")

	id, _, err := r.JobService.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{Status: job.StateError}, false, err
	}

	sigChan := r.registerInterruptOnSignal(id, opts.RealDevice, opts.DisplayName)
	defer unregisterSignalCapture(sigChan)

	r.uploadSauceConfig(id, opts.RealDevice, opts.ConfigFilePath)
	r.uploadCLIFlags(id, opts.RealDevice, opts.CLIFlags)

	// os.Interrupt can arrive before the signal.Notify() is registered. In that case,
	// if a soft exit is requested during startContainer phase, it gently exits.
	if r.interrupted {
		r.stopSuiteExecution(id, opts.RealDevice, opts.DisplayName)
		j, err = r.JobService.PollJob(context.Background(), id, 15*time.Second, opts.Timeout, opts.RealDevice)
		return j, true, err
	}

	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), id)
	l := log.Info().Str("url", jobDetailsPage).Str("suite", opts.DisplayName).Str("platform", opts.PlatformName)

	if opts.RealDevice {
		l.Str("deviceName", opts.DeviceName).Str("platformVersion", opts.PlatformVersion).Str("deviceId", opts.DeviceID)
		l.Bool("private", opts.DevicePrivateOnly)
	} else {
		l.Str("browser", opts.BrowserName)
	}

	l.Msg("Suite started.")

	// Async mode. Mark the job as started without waiting for the result.
	if r.Async {
		return job.Job{ID: id, IsRDC: opts.RealDevice, Status: job.StateInProgress}, false, nil
	}

	// High interval poll to not oversaturate the job reader with requests
	j, err = r.JobService.PollJob(context.Background(), id, 15*time.Second, opts.Timeout, opts.RealDevice)
	if err != nil {
		return job.Job{}, r.interrupted, fmt.Errorf("failed to retrieve job status for suite %s: %s", opts.DisplayName, err.Error())
	}

	// Enrich RDC data
	if opts.RealDevice {
		enrichRDCReport(&j, opts)
	}

	// Check timeout
	if j.TimedOut {
		color.Red("Suite '%s' has reached timeout of %s", opts.DisplayName, opts.Timeout)
		j, err = r.JobService.StopJob(context.Background(), id, opts.RealDevice)
		if err != nil {
			color.HiRedString("Failed to stop suite '%s': %v", opts.DisplayName, err)
		}
		j.Passed = false
		j.TimedOut = true
		return j, false, fmt.Errorf("suite '%s' has reached timeout", opts.DisplayName)
	}

	if !j.Passed {
		// We may need to differentiate when a job has crashed vs. when there is errors.
		return j, r.interrupted, fmt.Errorf("suite '%s' has test failures", opts.DisplayName)
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

		if opts.Attempt < opts.Retries && !jobData.Passed && !skipped {
			log.Warn().Err(err).Msg("Suite errored.")
			opts.Attempt++
			jobOpts <- opts
			log.Info().Str("suite", opts.DisplayName).Str("attempt", fmt.Sprintf("%d of %d", opts.Attempt+1, opts.Retries+1)).Msg("Retrying suite.")
			continue
		}

		if r.FailFast && !jobData.Passed {
			log.Warn().Err(err).Msg("FailFast mode enabled. Skipping upcoming suites.")
			r.interrupted = true
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

// remoteArchiveFolder archives the contents of the folder to a remote storage.
func (r CloudRunner) remoteArchiveFolder(project interface{}, folder string, sauceignoreFile string) (string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveFolder(project, tempDir, folder, sauceignoreFile)
	if err != nil {
		return "", err
	}

	return r.uploadProject(zipName, projectUpload)
}

// remoteArchiveFiles archives the files to a remote storage.
func (r CloudRunner) remoteArchiveFiles(project interface{}, files []string, sauceignoreFile string) (string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	zipName, err := r.archiveFiles(project, tempDir, files, sauceignoreFile)
	if err != nil {
		return "", err
	}

	return r.uploadProject(zipName, projectUpload)
}

func checkPathLength(projectFolder string, matcher sauceignore.Matcher) (string, error) {
	exampleName := ""
	maxLength := 0
	if err := filepath.Walk(projectFolder, func(file string, info fs.FileInfo, err error) error {
		if matcher.Match(strings.Split(file, string(os.PathSeparator)), info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if maxLength < len(file) {
			exampleName = file
			maxLength = len(file)
		}
		return nil
	}); err != nil {
		// When walk fails, we may not want to fail saucectl execution.
		return "", nil
	}

	if BaseFilepathLength+maxLength > MaxFilepathLength {
		return exampleName, errors.New("path too long")
	}
	return "", nil
}

// archiveFolder creates a zip file in the tempDir directory from the given projectFolder.
func (r *CloudRunner) archiveFolder(project interface{}, tempDir string, projectFolder string, sauceignoreFile string) (string, error) {
	matcher, err := r.getFileMatcher(projectFolder, sauceignoreFile)
	if err != nil {
		return "", err
	}

	if path, err := checkPathLength(projectFolder, matcher); err != nil {
		msg.PathTooLongForArchive(path)
		return "", err
	}

	start := time.Now()

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

	totalFileCount := 0
	for _, child := range folderContent {
		fileCount, err := z.Add(filepath.Join(projectFolder, child.Name()), "")
		if err != nil {
			return "", err
		}
		totalFileCount += fileCount
	}
	fileCount, err := z.Add(rcPath, "")
	if err != nil {
		return "", err
	}
	totalFileCount += fileCount

	err = z.Close()
	if err != nil {
		return "", err
	}

	f, err := os.Stat(zipName)
	if err != nil {
		return "", err
	}

	log.Info().Dur("durationMs", time.Since(start)).Int64("size", f.Size()).Int("fileCount", totalFileCount).Msg("Project archived.")
	if totalFileCount >= ArchiveFileCountSoftLimit {
		msg.LogArchiveSizeWarning()
	}

	return zipName, nil
}

// archiveFiles creates a zip file from the given files. Files retain their relative paths within the zip.
// Temporary files, as well as the zip itself, are created in the tempDir directory.
func (r *CloudRunner) archiveFiles(project interface{}, tempDir string, files []string, sauceignoreFile string) (string, error) {
	matcher, err := sauceignore.NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		return "", err
	}

	start := time.Now()

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

	totalFileCount := 0
	log.Debug().Str("name", rcPath).Msg("Adding to archive")
	fileCount, err := z.Add(rcPath, "")
	if err != nil {
		return "", err
	}
	totalFileCount += fileCount

	for _, f := range files {
		log.Debug().Str("name", f).Msg("Adding to archive")
		fileCount, err := z.Add(f, filepath.Dir(f))
		if err != nil {
			return "", err
		}
		totalFileCount += fileCount
	}

	err = z.Close()
	if err != nil {
		return "", err
	}

	f, err := os.Stat(zipName)
	if err != nil {
		return "", err
	}

	log.Info().Dur("durationMs", time.Since(start)).Int64("size", f.Size()).Int("fileCount", totalFileCount).Msg("Project archived.")
	if totalFileCount >= ArchiveFileCountSoftLimit {
		msg.LogArchiveSizeWarning()
	}

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

	if apps.IsRemote(filename) {
		log.Info().Msgf("Downloading from remote: %s", filename)

		progress.Show("Downloading %s", filename)
		dest, err := apps.Download(filename)
		progress.Stop()

		if err != nil {
			return "", err
		}
		defer os.RemoveAll(dest)

		filename = dest
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
	log.Info().Dur("durationMs", time.Since(start)).Str("storageId", resp.ID).
		Msgf("%s uploaded.", cases.Title(language.English).String(string(pType)))
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
		if res.job.Error != "" {
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

	// If a job errored (not to be confused with tests failing), there are likely no assets available anyway.
	if res.job.Error != "" {
		return
	}

	var assetContent []byte
	var err error

	// Display log only when at least it has started
	if assetContent, err = r.JobService.GetJobAssetFileContent(context.Background(), res.job.ID, ConsoleLogAsset, res.job.IsRDC); err == nil {
		log.Info().Str("suite", res.name).Msgf("console.log output: \n%s", assetContent)
		return
	}

	// Some frameworks produce a junit.xml instead, check for that file if there's no console.log
	assetContent, err = r.JobService.GetJobAssetFileContent(context.Background(), res.job.ID, "junit.xml", res.job.IsRDC)
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
	zipName, err := r.archiveFolder(project, tmpDir, folder, sauceIgnoreFile)
	if err != nil {
		return err
	}

	log.Info().Msgf("Saving bundled project to %s.", zipName)
	return nil
}

// stopSuiteExecution stops the current execution on Sauce Cloud
func (r *CloudRunner) stopSuiteExecution(jobID string, realDevice bool, suiteName string) {
	log.Info().Str("suite", suiteName).Msg("Stopping suite")
	_, err := r.JobService.StopJob(context.Background(), jobID, realDevice)
	if err != nil {
		log.Warn().Err(err).Str("suite", suiteName).Msg("Unable to stop suite.")
	}
}

// registerInterruptOnSignal stops execution on Sauce Cloud when a SIGINT is captured.
func (r *CloudRunner) registerInterruptOnSignal(jobID string, realDevice bool, suiteName string) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, jobID, suiteName string) {
		sig := <-c
		if sig == nil {
			return
		}
		r.stopSuiteExecution(jobID, realDevice, suiteName)
	}(sigChan, jobID, suiteName)
	return sigChan
}

// registerSkipSuitesOnSignal prevent new suites from being executed when a SIGINT is captured.
func (r *CloudRunner) registerSkipSuitesOnSignal() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
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
			println("\nStopping run. Waiting for all in progress tests to be stopped... (press Ctrl-c again to exit without waiting)\n")
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
func (r *CloudRunner) uploadSauceConfig(jobID string, realDevice bool, cfgFile string) {
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
	if err := r.JobService.UploadAsset(jobID, realDevice, filepath.Base(cfgFile), "text/plain", content); err != nil {
		log.Warn().Msgf("failed to attach configuration: %v", err)
	}
}

// uploadCLIFlags adds commandline parameters as an asset.
func (r *CloudRunner) uploadCLIFlags(jobID string, realDevice bool, content interface{}) {
	encoded, err := json.Marshal(content)
	if err != nil {
		log.Warn().Msgf("Failed to encode CLI flags: %v", err)
		return
	}
	if err := r.JobService.UploadAsset(jobID, realDevice, "flags.json", "text/plain", encoded); err != nil {
		log.Warn().Msgf("Failed to report CLI flags: %v", err)
	}
}

func (r *CloudRunner) deprecationMessage(frameworkName string, frameworkVersion string) string {
	return fmt.Sprintf(
		"%s%s%s",
		color.RedString(fmt.Sprintf("\nVersion %s for %s is deprecated and will be removed during our next framework release cycle !\n\n", frameworkVersion, frameworkName)),
		fmt.Sprintf("You should update your version of %s to a more recent one.\n", frameworkName),
		r.getAvailableVersionsMessage(frameworkName),
	)
}

func (r *CloudRunner) logFrameworkError(err error) {
	var unavailableErr *framework.FrameworkUnavailableError
	if errors.As(err, &unavailableErr) {
		color.Red(fmt.Sprintf("\n%s\n\n", err.Error()))
		fmt.Print(r.getAvailableVersionsMessage(unavailableErr.Name))
	}
}

// logAvailableVersions displays the available cloud version for the framework.
func (r *CloudRunner) getAvailableVersionsMessage(frameworkName string) string {
	versions, err := r.MetadataService.Versions(context.Background(), frameworkName)
	if err != nil {
		return ""
	}
	msg := fmt.Sprintf("Available versions of %s are:\n", frameworkName)
	for _, v := range versions {
		if !v.Deprecated {
			msg += fmt.Sprintf(" - %s\n", v.FrameworkVersion)
		}
	}
	msg += "\n"
	return msg
}

func (r *CloudRunner) getFileMatcher(projectFolder, sauceignoreFile string) (sauceignore.Matcher, error) {
	matcher, err := sauceignore.NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		return matcher, err
	}

	modDir := filepath.Join(projectFolder, "node_modules")
	modDirIgnored := matcher.Match([]string{modDir}, true)

	if modDirIgnored && len(r.NPMDependencies) > 0 {
		return matcher, fmt.Errorf("'node_modules' is ignored by sauceignore, but you have npm dependencies defined in your project; please remove 'node_modules' from your sauceignore file")
	}

	if !modDirIgnored {
		_, err := os.Stat(modDir)
		if err == nil && len(r.NPMDependencies) > 0 {
			log.Info().Msg("Picking up select dependencies from node_modules")
			patterns, _ := sauceignore.PatternsFromFile(sauceignoreFile)

			// by default, ignore everything under node_modules
			patterns = append(patterns, sauceignore.NewPattern("/node_modules/*"))

			// but un-ignore dependencies we want to keep
			reqs := node.Requirements(filepath.Join(projectFolder, "node_modules"), r.NPMDependencies...)
			log.Info().Msgf("Found a total of %d related npm dependencies", len(reqs))
			for _, req := range reqs {
				patterns = append(patterns, sauceignore.NewPattern("!/node_modules/"+req))
			}
			matcher = sauceignore.NewMatcher(patterns)
		}
	}

	return matcher, nil
}
