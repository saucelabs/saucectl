package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/msg"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/sauceignore"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/progress"
)

// ContainerRunner represents the container runner for docker.
type ContainerRunner struct {
	Ctx             context.Context
	docker          *Handler
	containerConfig *containerConfig
	Framework       framework.Framework
	FrameworkMeta   framework.MetadataService
	ShowConsoleLog  bool
}

// containerStartOptions represent data required to start a new container.
type containerStartOptions struct {
	Docker      config.Docker
	BeforeExec  []string
	Project     interface{}
	SuiteName   string
	Environment map[string]string
	Files       []string
	Sauceignore string
}

// result represents the result of a local job
type result struct {
	containerID   string
	err           error
	passed        bool
	consoleOutput string
	suiteName     string
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
	}

	if err := r.pullImage(docker.Image); err != nil {
		return err
	}
	return nil
}

func (r *ContainerRunner) startContainer(options containerStartOptions) (string, error) {
	container, err := r.docker.StartContainer(r.Ctx, options)
	if err != nil {
		return "", err
	}
	containerID := container.ID

	pDir, err := r.docker.ProjectDir(r.Ctx, options.Docker.Image)
	if err != nil {
		return "", err
	}

	r.containerConfig.jobInfoFilePath, err = r.docker.JobInfoFile(r.Ctx, options.Docker.Image)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "saucectl")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	rcPath := filepath.Join(tmpDir, SauceRunnerConfigFile)
	if err := jsonio.WriteFile(rcPath, options.Project); err != nil {
		return "", err
	}

	matcher, err := sauceignore.NewMatcherFromFile(options.Sauceignore)
	if err != nil {
		return "", err
	}

	if err := r.docker.CopyToContainer(r.Ctx, containerID, rcPath, pDir, matcher); err != nil {
		return "", err
	}
	r.containerConfig.sauceRunnerConfigPath = path.Join(pDir, SauceRunnerConfigFile)

	// running pre-exec tasks
	err = r.beforeExec(containerID, options.SuiteName, options.BeforeExec)
	if err != nil {
		return "", err
	}

	return container.ID, nil
}

func (r *ContainerRunner) run(containerID, suiteName string, cmd []string, env map[string]string) (output string, jobInfo jobInfo, passed bool, err error) {
	defer func() {
		log.Info().Str("suite", suiteName).Msg("Tearing down environment")
		if err := r.docker.Teardown(r.Ctx, containerID); err != nil {
			if !r.docker.IsErrNotFound(err) {
				log.Error().Err(err).Str("suite", suiteName).Msg("Failed to tear down environment")
			}
		}
	}()

	exitCode, output, err := r.docker.ExecuteAttach(r.Ctx, containerID, cmd, env)

	if err != nil {
		return "", jobInfo, false, err
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
	return output, jobInfo, passed, err
}

// readTestURL reads test url from inside the test runner container.
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
		containerID, output, jobDetails, passed, err := r.runSuite(opts)
		results <- result{
			suiteName:     opts.SuiteName,
			containerID:   containerID,
			jobInfo:       jobDetails,
			passed:        passed,
			consoleOutput: output,
			err:           err,
		}
	}
}

func (r *ContainerRunner) collectResults(results chan result, expected int) bool {
	// TODO find a better way to get the expected
	errCount := 0
	completed := 0
	inProgress := expected
	passed := true

	done := make(chan interface{})
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				break
			case <-t.C:
				log.Info().Msgf("Suites in progress: %d", inProgress)
			}
		}
	}()
	for i := 0; i < expected; i++ {
		res := <-results
		completed++
		inProgress--

		if !res.passed {
			errCount++
			passed = false
		}

		r.logSuite(res)
	}
	close(done)

	if errCount != 0 {
		msg.LogTestFailure(errCount, expected)
		return passed
	}

	msg.LogTestSuccess()

	return passed
}

func (r *ContainerRunner) logSuite(res result) {
	if res.containerID == "" {
		log.Error().Err(res.err).Str("suite", res.suiteName).Msg("Failed to start suite.")
		return
	}

	if res.passed {
		log.Info().Bool("passed", res.passed).Str("url", res.jobInfo.JobDetailsURL).Str("suite", res.suiteName).Msg("Suite finished.")
		if !res.jobInfo.ReportingSucceeded {
			log.Warn().Str("suite", res.suiteName).Msg("Reporting results to Sauce Labs failed.")
		}
	} else {
		log.Error().Bool("passed", res.passed).Str("url", res.jobInfo.JobDetailsURL).Str("suite", res.suiteName).Msg("Suite finished.")
	}

	if !res.passed || r.ShowConsoleLog {
		log.Info().Str("suite", res.suiteName).Msgf("console.log output: \n%s", res.consoleOutput)
	}
}

func (r *ContainerRunner) runSuite(options containerStartOptions) (containerID string, output string, jobInfo jobInfo, passed bool, err error) {
	log.Info().Str("suite", options.SuiteName).Msg("Setting up test environment")
	containerID, err = r.startContainer(options)
	if err != nil {
		log.Err(err).Str("suite", options.SuiteName).Msg("Failed to setup test environment")
		return
	}

	output, jobInfo, passed, err = r.run(containerID, options.SuiteName,
		[]string{"npm", "test", "--", "-r", r.containerConfig.sauceRunnerConfigPath, "-s", options.SuiteName},
		options.Environment)
	return
}
