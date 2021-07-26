package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/sauceignore"
)

// ContainerRunner represents the container runner for docker.
type ContainerRunner struct {
	Ctx               context.Context
	docker            *Handler
	containerConfig   *containerConfig
	Framework         framework.Framework
	FrameworkMeta     framework.MetadataService
	JobWriter         job.Writer
	ShowConsoleLog    bool
	JobReader         job.Reader
	ArtfactDownloader download.ArtifactDownloader

	interrupted bool
}

// containerStartOptions represent data required to start a new container.
type containerStartOptions struct {
	// DisplayName is used for local logging purposes only (e.g. console).
	DisplayName string

	// Timeout is used for local/per-suite timeout.
	Timeout time.Duration

	Docker         config.Docker
	BeforeExec     []string
	Project        interface{}
	SuiteName      string
	Browser        string
	Environment    map[string]string
	RootDir        string
	Sauceignore    string
	ConfigFilePath string
	CLIFlags       map[string]interface{}
}

// result represents the result of a local job
type result struct {
	containerID   string
	err           error
	passed        bool
	skipped       bool
	timedOut      bool
	consoleOutput string
	name          string
	browser       string
	duration      time.Duration
	jobInfo       jobInfo
}

// jobInfo represents the info on the job given by the container
type jobInfo struct {
	JobDetailsURL      string `json:"jobDetailsUrl"`
	ReportingSucceeded bool   `json:"reportingSucceeded"`
}

func (r *ContainerRunner) pullImage(img string) error {
	// Check docker image name property from the config file.
	if img == "" {
		return errors.New("no docker image specified")
	}

	// Check if image exists.
	hasImage, err := r.docker.HasBaseImage(r.Ctx, img)
	if err != nil {
		return err
	}

	// If it's our image, warn the user to not use the latest tag.
	if strings.HasPrefix(img, "saucelabs") && strings.HasSuffix(img, ":latest") {
		log.Warn().Msg("The use of 'latest' as the docker image tag is discouraged. " +
			"We recommend pinning the image to a specific version. " +
			"Please proceed with caution.")
	}

	// Only pull base image if not already installed.
	if !hasImage {
		progress.Show("Pulling image %s", img)
		defer progress.Stop()
		if err := r.docker.PullImage(r.Ctx, img); err != nil {
			return err
		}
	}

	return nil
}

// fetchImage ensure that container image is available for test runner to execute tests.
func (r *ContainerRunner) fetchImage(docker *config.Docker) error {
	if !r.docker.IsInstalled() {
		return fmt.Errorf("please verify that docker is installed and running: " +
			" follow the guide at https://docs.docker.com/get-docker/")
	}

	if docker.Image == "" {
		m, err := r.FrameworkMeta.Search(r.Ctx, framework.SearchOptions{
			Name:             r.Framework.Name,
			FrameworkVersion: r.Framework.Version,
		})
		if err != nil {
			return fmt.Errorf("unable to determine which docker image to run: %w", err)
		}
		docker.Image = m.DockerImage
	} else {
		log.Info().Msgf("Ignoring framework version for Docker, using provided image %s", docker.Image)
	}

	return r.pullImage(docker.Image)
}

func (r *ContainerRunner) startContainer(options containerStartOptions) (string, error) {
	container, err := r.docker.StartContainer(r.Ctx, options)
	if err != nil {
		return "", err
	}
	containerID := container.ID

	pDir, err := r.docker.ProjectDir(r.Ctx, options.Docker.Image)
	if err != nil {
		return containerID, err
	}

	r.containerConfig.jobInfoFilePath, err = r.docker.JobInfoFile(r.Ctx, options.Docker.Image)
	if err != nil {
		return containerID, err
	}

	tmpDir, err := os.MkdirTemp("", "saucectl")
	if err != nil {
		return containerID, err
	}
	defer os.RemoveAll(tmpDir)

	rcPath := filepath.Join(tmpDir, SauceRunnerConfigFile)
	if err := jsonio.WriteFile(rcPath, options.Project); err != nil {
		return containerID, err
	}

	matcher, err := sauceignore.NewMatcherFromFile(options.Sauceignore)
	if err != nil {
		return containerID, err
	}

	if err := r.docker.CopyToContainer(r.Ctx, containerID, rcPath, pDir, matcher); err != nil {
		return containerID, err
	}
	r.containerConfig.sauceRunnerConfigPath = path.Join(pDir, SauceRunnerConfigFile)

	// running pre-exec tasks
	err = r.beforeExec(containerID, options.SuiteName, options.BeforeExec)
	if err != nil {
		return containerID, err
	}

	return containerID, nil
}

func (r *ContainerRunner) run(containerID, suiteName string, cmd []string, env map[string]string, timeout time.Duration) (output string, jobInfo jobInfo, passed bool, timedOut bool, err error) {
	c := make(chan bool)

	var exitCode int
	go func(c chan bool) {
		exitCode, output, err = r.docker.ExecuteAttach(r.Ctx, containerID, cmd, env)
		c <- true
	}(c)

	if timeout <= 0 {
		timeout = 24 * time.Hour
	}
	deathclock := time.NewTimer(timeout)
	defer deathclock.Stop()

	select {
	case <-deathclock.C:
		timedOut = true
	case <-c:
	}

	if err != nil {
		return "", jobInfo, false, false, err
	}

	if timedOut {
		color.Red("Suite '%s' has reached timeout", suiteName)
		return "", jobInfo, false, true, fmt.Errorf("suite '%s' has reached timeout", suiteName)
	}

	passed = true
	if exitCode != 0 {
		log.Warn().Str("suite", suiteName).Msgf("exitCode is %d", exitCode)
		passed = false
	}

	jobInfo, err = r.readJobInfo(containerID)
	if err != nil {
		log.Warn().Msgf("unable to retrieve test result url: %s", err)
	}
	return output, jobInfo, passed, timedOut, err
}

// readJobInfo reads test url from inside the test runner container.
func (r *ContainerRunner) readJobInfo(containerID string) (jobInfo, error) {
	// Set unknown when image does not support it.
	if r.containerConfig.jobInfoFilePath == "" {
		return jobInfo{JobDetailsURL: "unknown"}, nil
	}
	dir, err := os.MkdirTemp("", "result")
	if err != nil {
		return jobInfo{}, err
	}
	defer os.RemoveAll(dir)

	err = r.docker.CopyFromContainer(r.Ctx, containerID, r.containerConfig.jobInfoFilePath, dir)
	if err != nil {
		return jobInfo{}, err
	}
	fileName := filepath.Base(r.containerConfig.jobInfoFilePath)
	filePath := filepath.Join(dir, fileName)
	content, err := os.ReadFile(filePath)

	var info jobInfo
	err = json.Unmarshal(content, &info)
	if err != nil {
		return jobInfo{}, err
	}
	return info, err
}

func (r *ContainerRunner) beforeExec(containerID, suiteName string, tasks []string) error {
	for _, task := range tasks {
		log.Info().Str("task", task).Str("suite", suiteName).Msg("Running BeforeExec")
		exitCode, _, err := r.docker.ExecuteAttach(r.Ctx, containerID, strings.Fields(task), nil)
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return fmt.Errorf("failed to run BeforeExec task: %s - exit code %d", task, exitCode)
		}
	}
	return nil
}

func (r *ContainerRunner) createWorkerPool(ccy int) (chan containerStartOptions, chan result) {
	jobOpts := make(chan containerStartOptions)
	results := make(chan result, ccy)

	log.Info().Int("concurrency", ccy).Msg("Launching workers.")

	for i := 0; i < ccy; i++ {
		go r.runJobs(jobOpts, results)
	}

	return jobOpts, results
}

func (r *ContainerRunner) runJobs(containerOpts <-chan containerStartOptions, results chan<- result) {
	for opts := range containerOpts {
		if r.interrupted {
			results <- result{
				name:    opts.DisplayName,
				skipped: true,
			}
			continue
		}
		start := time.Now()
		containerID, output, jobDetails, passed, skipped, timedOut, err := r.runSuite(opts)

		browser := fmt.Sprintf("%s %s", opts.Browser, r.docker.GetBrowserVersion(r.Ctx, opts.Docker.Image, opts.Browser))

		results <- result{
			name:          opts.DisplayName,
			containerID:   containerID,
			browser:       browser,
			jobInfo:       jobDetails,
			passed:        passed,
			skipped:       skipped,
			consoleOutput: output,
			duration:      time.Since(start),
			err:           err,
			timedOut:      timedOut,
		}
	}
}

func (r *ContainerRunner) collectResults(artifactCfg config.ArtifactDownload, results chan result, expected int) bool {
	// TODO find a better way to get the expected
	completed := 0
	inProgress := expected
	passed := true

	reporter := table.Reporter{
		TestResults: make([]report.TestResult, 0, expected),
		Dst:         os.Stdout,
	}

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
		completed++
		inProgress--

		jobID := getJobID(res.jobInfo.JobDetailsURL)
		if download.ShouldDownloadArtifact(jobID, res.passed, res.timedOut, artifactCfg) {
			r.ArtfactDownloader.DownloadArtifact(jobID)
		}

		if !res.passed {
			passed = false
		}

		if !res.skipped {
			reporter.Add(report.TestResult{
				Name:     res.name,
				Duration: res.duration,
				Passed:   res.passed,
				Browser:  res.browser,
				Platform: "Docker",
			})
		}

		r.logSuite(res)
	}
	close(done)

	reporter.Render()

	return passed
}

func getJobID(jobURL string) string {
	details := strings.Split(jobURL, "/")
	return details[len(details)-1]
}

func (r *ContainerRunner) logSuite(res result) {
	if res.skipped {
		log.Warn().Str("suite", res.name).Msg("Suite skipped.")
		return
	}
	if res.containerID == "" {
		log.Error().Err(res.err).Str("suite", res.name).Msg("Failed to start suite.")
		return
	}

	if res.passed {
		log.Info().Bool("passed", res.passed).Str("url", res.jobInfo.JobDetailsURL).Str("suite", res.name).Msg("Suite finished.")
		if !res.jobInfo.ReportingSucceeded {
			log.Warn().Str("suite", res.name).Msg("Reporting results to Sauce Labs failed.")
		}
	} else {
		log.Error().Bool("passed", res.passed).Str("url", res.jobInfo.JobDetailsURL).Str("suite", res.name).Msg("Suite finished.")
	}

	if !res.passed || r.ShowConsoleLog {
		log.Info().Str("suite", res.name).Msgf("console.log output: \n%s", res.consoleOutput)
	}
}

// runSuite runs the selected suite.
func (r *ContainerRunner) runSuite(options containerStartOptions) (containerID string, output string, jobInfo jobInfo, passed bool, skipped bool, timedOut bool, err error) {
	log.Info().Str("suite", options.DisplayName).Msg("Setting up test environment")
	containerID, err = r.startContainer(options)
	defer r.tearDown(containerID, options.SuiteName)

	// os.Interrupt can arrive before the signal.Notify() is registered. In that case,
	// if a soft exit is requested during startContainer phase, it gently exits.
	if r.interrupted {
		skipped = true
		return
	}

	sigC := r.registerInterruptOnSignal(containerID, options.SuiteName, &skipped)
	defer unregisterSignalCapture(sigC)

	if err != nil {
		log.Err(err).Str("suite", options.DisplayName).Msg("Failed to setup test environment")
		return
	}

	output, jobInfo, passed, timedOut, err = r.run(containerID, options.SuiteName,
		[]string{"npm", "test", "--", "-r", r.containerConfig.sauceRunnerConfigPath, "-s", options.SuiteName},
		options.Environment, options.Timeout)

	jobID := jobIDFromURL(jobIDFromURL(jobInfo.JobDetailsURL))
	if jobID != "" {
		r.uploadSauceConfig(jobID, options.ConfigFilePath)
		r.uploadCLIFlags(jobID, options.CLIFlags)
	}
	return
}

// uploadSauceConfig adds job configuration as an asset.
func (r *ContainerRunner) uploadSauceConfig(jobID string, cfgFile string) {
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
func (r *ContainerRunner) uploadCLIFlags(jobID string, content interface{}) {
	encoded, err := json.Marshal(content)
	if err != nil {
		log.Warn().Msgf("Failed to encode CLI flags: %v", err)
		return
	}
	if err := r.JobWriter.UploadAsset(jobID, "flags.json", "text/plain", encoded); err != nil {
		log.Warn().Msgf("Failed to report CLI flags: %v", err)
	}
}

// jobIDFromURL returns the jobID from the URL return by containers.
func jobIDFromURL(URL string) string {
	items := strings.Split(URL, "/")
	if len(items) < 1 {
		return ""
	}
	ID := items[len(items)-1]
	return ID
}

// registerInterruptOnSignal runs tearDown on SIGINT / Interrupt.
func (r *ContainerRunner) registerInterruptOnSignal(containerID, suiteName string, interrupted *bool) chan os.Signal {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, interrupted *bool, containerID, suiteName string) {
		sig := <-c
		if sig == nil {
			return
		}
		log.Info().Str("suite", suiteName).Msg("Interrupting suite")
		*interrupted = true
		r.tearDown(containerID, suiteName)
	}(sigChan, interrupted, containerID, suiteName)
	return sigChan
}

// registerSkipSuitesOnSignal prevent new suites from being executed when a SIGINT is captured.
func (r *ContainerRunner) registerSkipSuitesOnSignal() chan os.Signal {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, cr *ContainerRunner) {
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

// tearDown stops the test environment and remove docker containers.
func (r *ContainerRunner) tearDown(containerID, suiteName string) {
	if containerID == "" {
		return
	}
	log.Info().Str("suite", suiteName).Msg("Tearing down environment")
	if err := r.docker.Teardown(r.Ctx, containerID); err != nil {
		if !r.docker.IsErrNotFound(err) && !r.docker.IsErrRemovalInProgress(err) {
			log.Error().Err(err).Str("suite", suiteName).Msg("Failed to tear down environment")
		}
	}
}

// verifyFileTransferCompatibility will verify whether the configured FileTransfer docker settings are appropriate for
// the given concurrency. If not, it'll apply the config.DockerFileCopy and print out a message to notify the user.
func verifyFileTransferCompatibility(concurrency int, dockerConf *config.Docker) {
	if concurrency > 1 && dockerConf.FileTransfer != config.DockerFileCopy {
		log.Info().Msg("concurrency > 1: forcing file transfer mode to use 'copy'.")
		dockerConf.FileTransfer = config.DockerFileCopy
	}
}
