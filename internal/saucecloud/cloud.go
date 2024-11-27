package saucecloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/fatih/color"
	ptable "github.com/jedib0t/go-pretty/v6/table"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/apps"
	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/hashio"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/saucecloud/zip"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader        storage.AppService
	JobService             job.Service
	TunnelService          tunnel.Service
	Region                 region.Region
	MetadataService        framework.MetadataService
	ShowConsoleLog         bool
	Framework              framework.Framework
	MetadataSearchStrategy framework.MetadataSearchStrategy
	InsightsService        insights.Service
	UserService            iam.UserService
	BuildService           build.Service
	Retrier                retry.Retrier

	Reporters []report.Reporter

	Async    bool
	FailFast bool

	NPMDependencies []string

	interrupted bool
	Cache       Cache
}

type Cache struct {
	VDCBuild *build.Build
	RDCBuild *build.Build
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
	retries   int
	attempts  []report.Attempt

	details   insights.Details
	artifacts []report.Artifact
}

// ConsoleLogAsset represents job asset log file name.
const ConsoleLogAsset = "console.log"

func (r *CloudRunner) createWorkerPool(ccy int, maxRetries int) (chan job.StartOptions, chan result) {
	jobOpts := make(chan job.StartOptions, maxRetries+1)
	results := make(chan result, ccy)

	log.Info().Int("concurrency", ccy).Msg("Launching workers.")
	for i := 0; i < ccy; i++ {
		go r.runJobs(jobOpts, results)
	}

	return jobOpts, results
}

func (r *CloudRunner) collectResults(results chan result, expected int) bool {
	// TODO find a better way to get the expected
	completed := 0
	inProgress := expected
	passed := true

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
			platform := res.job.OS
			if res.job.OSVersion != "" {
				platform = fmt.Sprintf("%s %s", platform, res.job.OSVersion)
			}

			browser := res.browser
			// browser is empty for mobile tests
			if browser != "" {
				browser = fmt.Sprintf("%s %s", browser, res.job.BrowserVersion)
			}

			r.FetchJUnitReports(&res, res.artifacts)

			tr := report.TestResult{
				Name:       res.name,
				Duration:   res.duration,
				StartTime:  res.startTime,
				EndTime:    res.endTime,
				Status:     res.job.TotalStatus(),
				Browser:    browser,
				Platform:   platform,
				DeviceName: res.job.DeviceName,
				URL:        res.job.URL,
				Artifacts:  res.artifacts,
				Origin:     "sauce",
				RDC:        res.job.IsRDC,
				TimedOut:   res.job.TimedOut,
				Attempts:   res.attempts,
				BuildURL:   r.findBuild(res.job.ID, res.job.IsRDC).URL,
			}
			for _, rep := range r.Reporters {
				rep.Add(tr)
			}
		}
		r.logSuite(res)

		r.reportInsights(res)
	}
	close(done)

	if !r.interrupted {
		for _, rep := range r.Reporters {
			rep.Render()
		}
	}

	return passed
}

func (r *CloudRunner) findBuild(jobID string, isRDC bool) build.Build {
	if isRDC {
		if r.Cache.RDCBuild != nil {
			return *r.Cache.RDCBuild
		}
	} else {
		if r.Cache.VDCBuild != nil {
			return *r.Cache.VDCBuild
		}
	}

	b, err := r.BuildService.FindBuild(context.Background(), jobID, isRDC)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to retrieve build id for job (%s)", jobID)
		return build.Build{}
	}

	if isRDC {
		r.Cache.RDCBuild = &b
	} else {
		r.Cache.VDCBuild = &b
	}

	return b
}

func (r *CloudRunner) runJob(opts job.StartOptions) (j job.Job, skipped bool, err error) {
	log.Info().
		Str("suite", opts.DisplayName).
		Msg("Starting suite.")

	j, err = r.JobService.StartJob(context.Background(), opts)
	if err != nil {
		return job.Job{Status: job.StateError}, false, err
	}

	sigChan := r.registerInterruptOnSignal(j.ID, opts.RealDevice, opts.DisplayName)
	defer unregisterSignalCapture(sigChan)

	r.uploadSauceConfig(j.ID, opts.RealDevice, opts.ConfigFilePath)
	r.uploadCLIFlags(j.ID, opts.RealDevice, opts.CLIFlags)

	// os.Interrupt can arrive before the signal.Notify() is registered. In that case,
	// if a soft exit is requested during startContainer phase, it gently exits.
	if r.interrupted {
		r.stopSuiteExecution(j.ID, opts.RealDevice, opts.DisplayName)
		j, err = r.JobService.PollJob(context.Background(), j.ID, 15*time.Second, opts.Timeout, opts.RealDevice)
		return j, true, err
	}

	l := log.Info().Str("url", j.URL).Str("suite", opts.DisplayName).Str("platform", opts.PlatformName)

	if opts.RealDevice {
		l.Str("deviceName", opts.DeviceName).Str("platformVersion", opts.PlatformVersion).Str("deviceId", opts.DeviceID)
		l.Bool("private", opts.DevicePrivateOnly)
	} else {
		l.Str("browser", opts.BrowserName)
	}

	l.Msg("Suite started.")

	// Async mode. Return the current status without waiting for the final result.
	if r.Async {
		return j, false, nil
	}

	// High interval poll to not oversaturate the job reader with requests
	j, err = r.JobService.PollJob(context.Background(), j.ID, 15*time.Second, opts.Timeout, opts.RealDevice)
	if err != nil {
		return job.Job{}, r.interrupted, fmt.Errorf("failed to retrieve job status for suite %s: %s", opts.DisplayName, err.Error())
	}

	// Check timeout
	if j.TimedOut {
		log.Error().
			Str("suite", opts.DisplayName).
			Str("timeout", opts.Timeout.String()).
			Msg("Suite timed out.")

		r.stopSuiteExecution(j.ID, opts.RealDevice, opts.DisplayName)

		j.Passed = false
		j.TimedOut = true

		return j, false, errors.New("suite timed out")
	}

	if !j.Passed {
		// We may need to differentiate when a job has crashed vs. when there is errors.
		return j, r.interrupted, errors.New("suite has test failures")
	}

	return j, false, nil
}

func belowRetryLimit(opts job.StartOptions) bool {
	return opts.Attempt < opts.Retries
}

func belowThreshold(opts job.StartOptions) bool {
	return opts.CurrentPassCount < opts.PassThreshold
}

// shouldRetryJob checks if the job should be retried,
// based on whether it passed and if it was skipped.
func shouldRetryJob(jobData job.Job, skipped bool) bool {
	return !jobData.Passed && !skipped
}

// shouldRetry determines whether a job should be retried.
func (r *CloudRunner) shouldRetry(opts job.StartOptions, jobData job.Job, skipped bool) bool {
	return !r.Async && belowRetryLimit(opts) &&
		(shouldRetryJob(jobData, skipped) || belowThreshold(opts))
}

func (r *CloudRunner) runJobs(jobOpts chan job.StartOptions, results chan<- result) {
	for opts := range jobOpts {
		start := time.Now()

		details := insights.Details{
			Framework: opts.Framework,
			Browser:   opts.BrowserName,
			Tags:      opts.Tags,
			BuildName: opts.Build,
		}

		if r.interrupted {
			results <- result{
				name:     opts.DisplayName,
				browser:  opts.BrowserName,
				skipped:  true,
				err:      nil,
				attempts: opts.PrevAttempts,
				retries:  opts.Retries,
				details:  details,
			}
			continue
		}

		if opts.Attempt == 0 {
			opts.StartTime = start
		}

		jobData, skipped, err := r.runJob(opts)

		if jobData.Passed {
			opts.CurrentPassCount++
		}

		if r.shouldRetry(opts, jobData, skipped) {
			go r.JobService.DownloadArtifacts(jobData, false)
			if !jobData.Passed {
				log.Warn().Err(err).
					Str("attempt",
						fmt.Sprintf("%d of %d", opts.Attempt+1, opts.Retries+1),
					).
					Msg("Suite attempt failed.")
			}

			opts.Attempt++
			opts.PrevAttempts = append(opts.PrevAttempts, report.Attempt{
				ID:         jobData.ID,
				Duration:   time.Since(start),
				StartTime:  start,
				EndTime:    time.Now(),
				Status:     jobData.Status,
				TestSuites: junit.TestSuites{},
			})
			go r.Retrier.Retry(jobOpts, opts, jobData)

			continue
		}

		if r.FailFast && !jobData.Passed {
			log.Warn().Err(err).Msg("FailFast mode enabled. Skipping upcoming suites.")
			r.interrupted = true
		}

		if !r.Async {
			if opts.CurrentPassCount < opts.PassThreshold {
				log.Error().Str("suite", opts.DisplayName).Msg("Failed to pass threshold")
				jobData.Status = job.StateFailed
				jobData.Passed = false
			} else {
				log.Info().Str("suite", opts.DisplayName).Msg("Passed threshold")
				jobData.Status = job.StatePassed
				jobData.Passed = true
			}
		}

		files := r.JobService.DownloadArtifacts(jobData, true)
		var artifacts []report.Artifact
		for _, f := range files {
			artifacts = append(artifacts, report.Artifact{
				FilePath: f,
			})
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
			retries:   opts.Retries,
			details:   details,
			attempts: append(opts.PrevAttempts, report.Attempt{
				ID:        jobData.ID,
				Duration:  time.Since(opts.StartTime),
				StartTime: opts.StartTime,
				EndTime:   time.Now(),
				Status:    jobData.Status,
			}),
			artifacts: artifacts,
		}
	}
}

// remoteArchiveProject archives the contents of the folder and uploads to remote storage.
// Returns the app URI for the uploaded project and additional URIs for the
// runner config, node_modules, and other resources.
func (r *CloudRunner) remoteArchiveProject(project interface{}, projectDir string, sauceignoreFile string, dryRun bool) (app string, otherApps []string, err error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if err != nil {
		return
	}
	if !dryRun {
		defer os.RemoveAll(tempDir)
	}

	files, err := collectFiles(projectDir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to collect project files: %w", err)
	}

	matcher, err := sauceignore.NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		return
	}

	// Create archives for the project's main files and runner configuration.
	archives, err := r.createArchives(tempDir, projectDir, project, files, matcher)
	if err != nil {
		return
	}

	uris, err := r.uploadFiles(archives, dryRun)
	if err != nil {
		return
	}

	need, err := needsNodeModules(projectDir, matcher, r.NPMDependencies)
	if err != nil {
		return
	}
	if need {
		nodeModulesURI, err := r.handleNodeModules(tempDir, projectDir, matcher, dryRun)
		if err != nil {
			return "", nil, err
		}
		if nodeModulesURI != "" {
			uris[nodeModulesUpload] = nodeModulesURI
		}
	}

	var sortedURIs []string
	for _, t := range []uploadType{runnerConfigUpload, nodeModulesUpload, otherAppsUpload} {
		if uri, ok := uris[t]; ok {
			sortedURIs = append(sortedURIs, uri)
		}
	}

	return uris[projectUpload], sortedURIs, nil
}

// collectFiles retrieves all relevant files in the project directory, excluding "node_modules".
func collectFiles(dir string) ([]string, error) {
	var files []string
	contents, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read project directory: %w", err)
	}

	for _, file := range contents {
		if file.Name() != "node_modules" {
			files = append(files, filepath.Join(dir, file.Name()))
		}
	}
	return files, nil
}

func (r *CloudRunner) createArchives(tempDir, projectDir string, project interface{}, files []string, matcher sauceignore.Matcher) (map[uploadType]string, error) {
	archives := make(map[uploadType]string)

	projectArchive, err := zip.ArchiveFiles("app", tempDir, projectDir, files, matcher)
	if err != nil {
		return nil, fmt.Errorf("failed to archive project files: %w", err)
	}
	archives[projectUpload] = projectArchive

	configArchive, err := zip.ArchiveRunnerConfig(project, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to archive runner configuration: %w", err)
	}
	archives[runnerConfigUpload] = configArchive

	return archives, nil
}

// handleNodeModules archives the node_modules directory and uploads it to remote storage.
// Checks if npm dependencies are taggable and if a tagged version of node_modules already exists in storage.
// If an existing archive is found, it returns the URI of that archive.
// If not, it creates a new archive, uploads it, and returns the storage ID.
func (r *CloudRunner) handleNodeModules(tempDir, projectDir string, matcher sauceignore.Matcher, dryRun bool) (string, error) {
	var tags []string

	if taggableModules(projectDir, r.NPMDependencies) {
		tag, err := hashio.HashContent(filepath.Join(projectDir, "package-lock.json"), r.NPMDependencies...)
		if err != nil {
			return "", err
		}
		tags = append(tags, tag)

		log.Info().Msgf("Searching remote node_modules archive by tag %s", tag)
		existingURI := r.findTaggedArchives(tag)
		if existingURI != "" {
			log.Info().Msgf("Skipping upload, using %s", existingURI)
			return existingURI, nil
		}
	}

	archive, err := zip.ArchiveNodeModules(tempDir, projectDir, matcher, r.NPMDependencies)
	if err != nil {
		return "", fmt.Errorf("failed to archive node_modules: %w", err)
	}

	return r.uploadArchive(storage.FileInfo{Name: archive, Tags: tags}, nodeModulesUpload, dryRun)
}

func needsNodeModules(projectDir string, matcher sauceignore.Matcher, dependencies []string) (bool, error) {
	modDir := filepath.Join(projectDir, "node_modules")
	ignored := matcher.Match(strings.Split(modDir, string(os.PathSeparator)), true)
	hasMods := fileExists(modDir)
	wantMods := len(dependencies) > 0

	if wantMods && !hasMods {
		return false, fmt.Errorf("unable to access 'node_modules' folder, but you have npm dependencies defined in your configuration; ensure that the folder exists and is accessible")
	}

	if ignored && wantMods {
		return false, fmt.Errorf("'node_modules' is ignored by sauceignore, but you have npm dependencies defined in your project; please remove 'node_modules' from your sauceignore file")
	}

	if !hasMods || ignored {
		return false, nil
	}

	return true, nil
}

// taggableModules checks if tagging should be applied based on the presence of package-lock.json and dependencies.
func taggableModules(dir string, npmDependencies []string) bool {
	return len(npmDependencies) > 0 && fileExists(filepath.Join(dir, "package-lock.json"))
}

// findTaggedArchives searches storage for a tagged archive with a matching tag.
func (r *CloudRunner) findTaggedArchives(tag string) string {
	list, err := r.ProjectUploader.List(context.TODO(), storage.ListOptions{Tags: []string{tag}, MaxResults: 1})
	if err != nil {
		log.Err(err).Msgf("Failed to retrieve file with tag %q from storage", tag)
		return ""
	}
	if len(list.Items) == 0 {
		return ""
	}

	return fmt.Sprintf("storage:%s", list.Items[0].ID)
}

// uploadFiles uploads each archive and returns a map of URIs.
func (r *CloudRunner) uploadFiles(archives map[uploadType]string, dryRun bool) (map[uploadType]string, error) {
	uris := make(map[uploadType]string)
	for uploadType, path := range archives {
		uri, err := r.uploadArchive(storage.FileInfo{Name: path}, uploadType, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to upload %s archive: %w", uploadType, err)
		}
		uris[uploadType] = uri
	}
	return uris, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// remoteArchiveFiles archives the files to a remote storage.
func (r *CloudRunner) remoteArchiveFiles(project interface{}, files []string, sauceignoreFile string, dryRun bool) (string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if err != nil {
		return "", err
	}
	if !dryRun {
		defer os.RemoveAll(tempDir)
	}

	archives := make(map[uploadType]string)

	matcher, err := sauceignore.NewMatcherFromFile(sauceignoreFile)
	if err != nil {
		return "", err
	}

	zipName, err := zip.ArchiveFiles("app", tempDir, ".", files, matcher)
	if err != nil {
		return "", err
	}
	archives[projectUpload] = zipName

	configZip, err := zip.ArchiveRunnerConfig(project, tempDir)
	if err != nil {
		return "", err
	}
	archives[runnerConfigUpload] = configZip

	var uris []string
	for k, v := range archives {
		uri, err := r.uploadArchive(storage.FileInfo{Name: v}, k, dryRun)
		if err != nil {
			return "", err
		}
		uris = append(uris, uri)

	}

	return strings.Join(uris, ","), nil
}

// FetchJUnitReports retrieves junit reports for the given result and all of its
// attempts. Can use the given artifacts to avoid unnecessary API calls.
func (r *CloudRunner) FetchJUnitReports(res *result, artifacts []report.Artifact) {
	if !report.IsArtifactRequired(r.Reporters, report.JUnitArtifact) {
		return
	}

	var junitArtifact *report.Artifact
	for _, artifact := range artifacts {
		if strings.HasSuffix(artifact.FilePath, junit.FileName) {
			junitArtifact = &artifact
			break
		}
	}

	for i := range res.attempts {
		attempt := &res.attempts[i]

		var content []byte
		var err error

		// If this is the last attempt, we can use the given junit artifact to
		// avoid unnecessary API calls.
		if i == len(res.attempts)-1 && junitArtifact != nil {
			content, err = os.ReadFile(junitArtifact.FilePath)
			log.Debug().Msg("Using cached JUnit report")
		} else {
			content, err = r.JobService.Artifact(
				context.Background(),
				attempt.ID,
				junit.FileName,
				res.job.IsRDC,
			)
		}

		if err != nil {
			log.Warn().Err(err).Str("jobID", attempt.ID).Msg("Unable to retrieve JUnit report")
			continue
		}

		attempt.TestSuites, err = junit.Parse(content)
		if err != nil {
			log.Warn().Err(err).Str("jobID", attempt.ID).Msg("Unable to parse JUnit report")
			continue
		}
	}
}

type uploadType string

var (
	testAppUpload      uploadType = "test application"
	appUpload          uploadType = "application"
	projectUpload      uploadType = "project"
	runnerConfigUpload uploadType = "runner config"
	nodeModulesUpload  uploadType = "node modules"
	otherAppsUpload    uploadType = "other applications"
)

func (r *CloudRunner) uploadArchives(filenames []string, pType uploadType, dryRun bool) ([]string, error) {
	var IDs []string
	for _, f := range filenames {
		ID, err := r.uploadArchive(storage.FileInfo{Name: f}, pType, dryRun)
		if err != nil {
			return []string{}, err
		}
		IDs = append(IDs, ID)
	}

	return IDs, nil
}

func (r *CloudRunner) uploadArchive(fileInfo storage.FileInfo, pType uploadType, dryRun bool) (string, error) {
	filename := fileInfo.Name
	if dryRun {
		log.Info().Str("file", filename).Msgf("Skipping upload in dry run.")
		return "", nil
	}

	if apps.IsStorageReference(filename) {
		return apps.NormalizeStorageReference(filename), nil
	}

	if apps.IsRemote(filename) {
		log.Info().Msgf("Downloading from remote: %s", filename)

		progress.Show("Downloading %s", filename)
		dest, err := r.download(filename)
		progress.Stop()
		if err != nil {
			return "", fmt.Errorf("unable to download app from %s: %w", filename, err)
		}
		defer os.RemoveAll(dest)

		filename = dest
	}

	log.Info().Msgf("Checking if %s has already been uploaded previously", filename)
	if storageID, _ := r.isFileStored(filename); storageID != "" {
		log.Info().Msgf("Skipping upload, using storage:%s", storageID)
		return fmt.Sprintf("storage:%s", storageID), nil
	}

	filename, err := filepath.Abs(filename)
	if err != nil {
		return "", nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("project upload: %w", err)
	}
	defer file.Close()

	progress.Show("Uploading %s %s", pType, filename)
	start := time.Now()
	resp, err := r.ProjectUploader.UploadStream(
		context.TODO(),
		storage.FileInfo{
			Name:        filepath.Base(filename),
			Description: fileInfo.Description,
			Tags:        fileInfo.Tags,
		},
		file,
	)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().
		Str("duration", time.Since(start).Round(time.Second).String()).
		Str("storageId", resp.ID).
		Msgf("%s uploaded.", cases.Title(language.English).String(string(pType)))
	return fmt.Sprintf("storage:%s", resp.ID), nil
}

// isFileStored calculates the checksum of the given file and looks up its existence in the Sauce Labs app storage.
// Returns an empty string if no file was found.
func (r *CloudRunner) isFileStored(filename string) (storageID string, err error) {
	hash, err := hashio.SHA256(filename)
	if err != nil {
		return "", err
	}

	log.Info().Msgf("Checksum: %s", hash)

	l, err := r.ProjectUploader.List(context.TODO(), storage.ListOptions{
		SHA256:     hash,
		MaxResults: 1,
	})
	if err != nil {
		return "", err
	}
	if len(l.Items) == 0 {
		return "", nil
	}

	return l.Items[0].ID, nil
}

// logSuite display the result of a suite
func (r *CloudRunner) logSuite(res result) {
	logger := log.With().
		Str("suite", res.name).
		Bool("passed", res.job.Passed).
		Str("url", res.job.URL).
		Logger()

	if res.err != nil {
		if res.skipped {
			logger.Error().Err(res.err).Msg("Suite skipped.")
			return
		}
		if res.job.ID == "" {
			logger.Error().Err(res.err).Msg("Suite failed to start.")
			return
		}
		logger.Error().Err(res.err).Msg("Suite failed unexpectedly.")
		return
	}

	// Job isn't done, hence nothing more to log about it.
	if !job.Done(res.job.Status) || r.Async {
		return
	}

	if res.job.TimedOut {
		logger.Error().Msg("Suite timed out.")
		return
	}

	if res.job.Passed {
		logger.Info().Msg("Suite passed.")
	} else {
		l := logger.Error()
		if res.job.Error != "" {
			l.Str("error", res.job.Error)
		}
		l.Msg("Suite failed.")
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
	if assetContent, err = r.JobService.Artifact(context.Background(), res.job.ID, ConsoleLogAsset, res.job.IsRDC); err == nil {
		log.Info().Str("suite", res.name).Msgf("console.log output: \n%s", assetContent)
		return
	}

	// Some frameworks produce a junit.xml instead, check for that file if there's no console.log
	assetContent, err = r.JobService.Artifact(context.Background(), res.job.ID, junit.FileName, res.job.IsRDC)
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
			if tc.Error != nil {
				fmt.Printf("\n\t%d) %s.%s\n\n", errCount, tc.ClassName, tc.Name)
				headerColor.Println("\tError was:")
				bodyColor.Printf("\t%s\n", tc.Error)
				errCount++
			} else if tc.Failure != nil {
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

func (r *CloudRunner) validateTunnel(name, owner string, dryRun bool, timeout time.Duration) error {
	return tunnel.Validate(r.TunnelService, name, owner, tunnel.NoneFilter, dryRun, timeout)
}

// stopSuiteExecution stops the current execution on Sauce Cloud
func (r *CloudRunner) stopSuiteExecution(jobID string, realDevice bool, suiteName string) {
	log.Info().Str("suite", suiteName).Msg("Attempting to stop job...")

	// Ignore errors when stopping a job, as it may have already ended or is in
	// a state where it cannot be stopped. Either way, there's nothing we can do.
	_, _ = r.JobService.StopJob(context.Background(), jobID, realDevice)
}

// registerInterruptOnSignal stops execution on Sauce Cloud when a SIGINT is captured.
func (r *CloudRunner) registerInterruptOnSignal(jobID string, realDevice bool, suiteName string) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		sig := <-sigChan
		if sig == nil {
			return
		}
		r.stopSuiteExecution(jobID, realDevice, suiteName)
	}()

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
	if err := r.JobService.UploadArtifact(jobID, realDevice, filepath.Base(cfgFile), "text/plain", content); err != nil {
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
	if err := r.JobService.UploadArtifact(jobID, realDevice, "flags.json", "text/plain", encoded); err != nil {
		log.Warn().Msgf("Failed to report CLI flags: %v", err)
	}
}

func (r *CloudRunner) logFrameworkError(err error) {
	var unavailableErr *framework.UnavailableError
	if errors.As(err, &unavailableErr) {
		color.Red(fmt.Sprintf("\n%s\n\n", err.Error()))
		fmt.Print(msg.FormatAvailableVersions(unavailableErr.Name, r.getAvailableVersions(unavailableErr.Name)))
	}
}

// getAvailableVersions gets the available cloud version for the framework.
func (r *CloudRunner) getAvailableVersions(frameworkName string) []string {
	versions, err := r.MetadataService.Versions(context.Background(), frameworkName)
	if err != nil {
		return nil
	}

	var available []string
	for _, v := range versions {
		if !v.IsDeprecated() && !v.IsFlaggedForRemoval() {
			available = append(available, v.FrameworkVersion)
		}
	}
	return available
}

func (r *CloudRunner) getHistory(launchOrder config.LaunchOrder) (insights.JobHistory, error) {
	user, err := r.UserService.User(context.Background())
	if err != nil {
		return insights.JobHistory{}, err
	}

	// The config uses spaces, but the API requires underscores.
	sortBy := strings.ReplaceAll(string(launchOrder), " ", "_")

	return r.InsightsService.GetHistory(context.Background(), user, sortBy)
}

func (r *CloudRunner) reportInsights(res result) {
	// NOTE: Jobs must be finished in order to be reported to Insights.
	// * Async jobs have an unknown status by definition, so should always be excluded from reporting.
	// * Timed out jobs will be requested to stop, but stopping a job
	//   is either not possible (rdc) or async (vdc) so its actual status is not known now.
	//   Skip reporting to be safe.
	if r.Async || !job.Done(res.job.Status) || res.job.TimedOut || res.skipped || res.job.ID == "" {
		return
	}

	res.details.BuildID = r.findBuild(res.job.ID, res.job.IsRDC).ID

	assets, err := r.JobService.ArtifactNames(context.Background(), res.job.ID, res.job.IsRDC)
	if err != nil {
		log.Warn().Err(err).Str("action", "loadAssets").Str("jobID", res.job.ID).Msg(msg.InsightsReportError)
		return
	}

	// read job from insights to get accurate platform and device name
	j, err := r.InsightsService.ReadJob(context.Background(), res.job.ID)
	if err != nil {
		log.Warn().Err(err).Str("action", "readJob").Str("jobID", res.job.ID).Msg(msg.InsightsReportError)
		return
	}
	res.details.Platform = strings.TrimSpace(fmt.Sprintf("%s %s", j.OS, j.OSVersion))
	res.details.Device = j.DeviceName

	var testRuns []insights.TestRun
	if arrayContains(assets, saucereport.FileName) {
		report, err := r.loadSauceTestReport(res.job.ID, res.job.IsRDC)
		if err != nil {
			log.Warn().Err(err).Str("action", "parsingJSON").Str("jobID", res.job.ID).Msg(msg.InsightsReportError)
			return
		}
		testRuns = insights.FromSauceReport(report, res.job.ID, res.name, res.details, res.job.IsRDC)
	} else if arrayContains(assets, junit.FileName) {
		report, err := r.loadJUnitReport(res.job.ID, res.job.IsRDC)
		if err != nil {
			log.Warn().Err(err).Str("action", "parsingXML").Str("jobID", res.job.ID).Msg(msg.InsightsReportError)
			return
		}
		testRuns = insights.FromJUnit(report, res.job.ID, res.name, res.details, res.job.IsRDC)
	}

	if len(testRuns) > 0 {
		if err := r.InsightsService.PostTestRun(context.Background(), testRuns); err != nil {
			log.Warn().Err(err).Str("action", "posting").Str("jobID", res.job.ID).Msg(msg.InsightsReportError)
		}
	}
}

func (r *CloudRunner) loadSauceTestReport(jobID string, isRDC bool) (saucereport.SauceReport, error) {
	fileContent, err := r.JobService.Artifact(context.Background(), jobID, saucereport.FileName, isRDC)
	if err != nil {
		log.Warn().Err(err).Str("action", "loading-json-report").Msg(msg.InsightsReportError)
		return saucereport.SauceReport{}, err
	}
	return saucereport.Parse(fileContent)
}

func (r *CloudRunner) loadJUnitReport(jobID string, isRDC bool) (junit.TestSuites, error) {
	fileContent, err := r.JobService.Artifact(context.Background(), jobID, junit.FileName, isRDC)
	if err != nil {
		log.Warn().Err(err).Str("action", "loading-xml-report").Msg(msg.InsightsReportError)
		return junit.TestSuites{}, err
	}
	return junit.Parse(fileContent)
}

func arrayContains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// download downloads the resource the URL points to and returns its local path.
func (r *CloudRunner) download(url string) (string, error) {
	reader, _, err := r.ProjectUploader.DownloadURL(context.TODO(), url)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	dir, err := os.MkdirTemp("", "tmp-app")
	if err != nil {
		return "", err
	}

	tmpFilePath := path.Join(dir, path.Base(url))

	f, err := os.Create(tmpFilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)

	return tmpFilePath, err
}

func printDryRunSuiteNames(suites []string) {
	fmt.Println("\nThe following test suites would have run:")
	for _, s := range suites {
		fmt.Printf("  - %s\n", s)
	}
	fmt.Println()
}
