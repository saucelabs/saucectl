package docker

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/dots"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/progress"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ContainerRunner represents the container runner for docker.
type ContainerRunner struct {
	Ctx             context.Context
	Cli             *command.SauceCtlCli
	docker          *Handler
	containerConfig *containerConfig
	Framework       framework.Framework
	ImageLoc        framework.ImageLocator
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
}

// result represents the result of a local job
type result struct {
	containerID   string
	err           error
	passed        bool
	output        string
	suiteName     string
	jobDetailsURL string
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
		img, err := r.ImageLoc.GetImage(r.Ctx, r.Framework)
		if err != nil {
			return fmt.Errorf("unable to determine which docker image to run: %w", err)
		}
		docker.Image = img
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

	r.containerConfig.jobDetailsFilePath, err = r.docker.JobDetailsURLFile(r.Ctx, options.Docker.Image)
	if err != nil {
		return "", err
	}

	tmpDir, err := ioutil.TempDir("", "saucectl")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	rcPath := filepath.Join(tmpDir, SauceRunnerConfigFile)
	if err := jsonio.WriteFile(rcPath, options.Project); err != nil {
		return "", err
	}

	if err := r.docker.CopyToContainer(r.Ctx, containerID, rcPath, pDir); err != nil {
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

func (r *ContainerRunner) run(containerID, suiteName string, cmd []string, env map[string]string) (string, string, string, bool, error) {
	defer func() {
		log.Info().Msgf("%s: Tearing down environment", suiteName)
		if err := r.docker.Teardown(r.Ctx, containerID); err != nil {
			if !r.docker.IsErrNotFound(err) {
				log.Error().Err(err).Msgf("%s: Failed to tear down environment", suiteName)
			}
		}
	}()

	exitCode, output, err := r.docker.ExecuteAttach(r.Ctx, containerID, cmd, env)

	if err != nil {
		return "", "", "", false, err
	}

	passed := true
	if exitCode != 0 {
		err = fmt.Errorf("exitCode is %d", exitCode)
		passed = false
	}

	jobDetailsURL, err := r.readTestURL(containerID)
	if err != nil {
		log.Warn().Msgf("unable to retrieve test result url: %s", err)
	}
	return containerID, output, jobDetailsURL, passed, nil
}

// readTestUrl reads test url from inside the test runner container.
func (r *ContainerRunner) readTestURL(containerID string) (string, error) {
	dir, err := ioutil.TempDir("", "result")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	err = r.docker.CopyFromContainer(r.Ctx, containerID, r.containerConfig.jobDetailsFilePath, dir)
	if err != nil {
		return "", err
	}
	fileName := filepath.Base(r.containerConfig.jobDetailsFilePath)
	filePath := filepath.Join(dir, fileName)
	content, err := ioutil.ReadFile(filePath)
	return strings.TrimSpace(string(content)), err
}

func (r *ContainerRunner) beforeExec(containerID, suiteName string, tasks []string) error {
	for _, task := range tasks {
		log.Info().Str("task", task).Msgf("%s: Running BeforeExec", suiteName)
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
		containerID, output, jobDetailsURL, passed, err := r.runSuite(opts)
		results <- result{
			suiteName:     opts.SuiteName,
			containerID:   containerID,
			jobDetailsURL: jobDetailsURL,
			passed:        passed,
			output:        output,
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

	waiter := dots.New(1)
	waiter.Start()
	for i := 0; i < expected; i++ {
		res := <-results
		completed++
		inProgress--

		if !res.passed {
			errCount++
			passed = false
		}

		// Logging is not synchronized over the different worker routines & dot routine.
		// To avoid implementing a more complex solution centralizing output on only one
		// routine, a new lines has simply been forced, to ensure that line starts from
		// the beginning of the console.
		fmt.Println("")
		log.Info().Msgf("Suites completed: %d/%d", completed, expected)
		r.logSuite(res)
	}
	waiter.Stop()

	log.Info().Msgf("Suites expected: %d", expected)
	log.Info().Msgf("Suites passed: %d", expected-errCount)
	log.Info().Msgf("Suites failed: %d", errCount)

	return passed
}

func (r *ContainerRunner) logSuite(res result) {
	if res.containerID == "" {
		log.Error().Err(res.err).Msgf("%s: Failed to start suite.", res.suiteName)
		return
	}

	if res.passed {
		log.Info().Bool("passed", res.passed).Str("url", res.jobDetailsURL).Msgf("%s: Suite finished.", res.suiteName)
	} else {
		log.Error().Bool("passed", res.passed).Str("url", res.jobDetailsURL).Msgf("%s: Suite finished.", res.suiteName)
	}

	if !res.passed || r.ShowConsoleLog {
		log.Info().Msgf("%s: console.log output: \n%s", res.suiteName, res.output)
	}
}

func (r *ContainerRunner) runSuite(options containerStartOptions) (string, string, string, bool, error) {
	log.Info().Msgf("%s: Setting up test environment", options.SuiteName)
	containerID, err := r.startContainer(options)
	if err != nil {
		log.Err(err).Msgf("%s: Failed to setup test environment", options.SuiteName)
		return containerID, "", "", false, err
	}

	return r.run(containerID, options.SuiteName,
		[]string{"npm", "test", "--", "-r", r.containerConfig.sauceRunnerConfigPath, "-s", options.SuiteName},
		options.Environment)
}
