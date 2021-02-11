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
	err    error
	passed bool
	output string
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

// setupImage performs any necessary steps for a test runner to execute tests.
func (r *ContainerRunner) setupImage(confd config.Docker, beforeExec []string, project interface{}, files []string) (string, error) {
	if !r.docker.IsInstalled() {
		return "", fmt.Errorf("please verify that docker is installed and running: " +
			" follow the guide at https://docs.docker.com/get-docker/")
	}

	if confd.Image == "" {
		img, err := r.ImageLoc.GetImage(r.Ctx, r.Framework)
		if err != nil {
			return "", fmt.Errorf("unable to determine which docker image to run: %w", err)
		}
		confd.Image = img
	}

	if err := r.pullImage(confd.Image); err != nil {
		return "", err
	}

	container, err := r.docker.StartContainer(r.Ctx, files, confd)
	if err != nil {
		return "", err
	}
	containerID := container.ID

	pDir, err := r.docker.ProjectDir(r.Ctx, confd.Image)
	if err != nil {
		return "", err
	}

	tmpDir, err := ioutil.TempDir("", "saucectl")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	rcPath := filepath.Join(tmpDir, SauceRunnerConfigFile)
	if err := jsonio.WriteFile(rcPath, project); err != nil {
		return "", err
	}

	if err := r.docker.CopyToContainer(r.Ctx, containerID, rcPath, pDir); err != nil {
		return "", err
	}
	r.containerConfig.sauceRunnerConfigPath = path.Join(pDir, SauceRunnerConfigFile)

	// running pre-exec tasks
	err = r.beforeExec(containerID, beforeExec)
	if err != nil {
		return "", err
	}

	return container.ID, nil
}

func (r *ContainerRunner) run(containerID string, cmd []string, env map[string]string) (bool, string, error) {
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err := r.docker.Teardown(r.Ctx, containerID); err != nil {
			if !r.docker.IsErrNotFound(err) {
				log.Error().Err(err).Msg("Failed to tear down environment")
			}
		}
	}()

	exitCode, output, err := r.docker.ExecuteAttach(r.Ctx, containerID, r.Cli, cmd, env)
	// FIXME: to restore somewhere else
	//log.Info().
	//	Int("ExitCode", exitCode).
	//	Msg("Command Finished")

	if err != nil {
		return false, "", err
	}
	if exitCode != 0 {
		return false, "", fmt.Errorf("exitCode is %d", exitCode)
	}
	passed := exitCode == 0
	return passed, output, nil
}

func (r *ContainerRunner) beforeExec(containerID string, tasks []string) error {
	for _, task := range tasks {
		log.Info().Str("task", task).Msg("Running BeforeExec")
		exitCode, _, err := r.docker.ExecuteAttach(r.Ctx, containerID, r.Cli, strings.Fields(task), nil)
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
		passed, output, err := r.runSuite(opts)
		results <- result{
			passed: passed,
			output: output,
			err:    err,
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
}

func (r *ContainerRunner) runSuite(options containerStartOptions) (bool, string, error) {
	log.Info().Msg("Setting up test environment")
	containerID, err := r.setupImage(options.Docker, options.BeforeExec, options.Project, options.Files)
	if err != nil {
		log.Err(err).Msg("Failed to setup test environment")
		return false, "", err
	}

	return r.run(containerID, []string{"npm", "test", "--", "-r", r.containerConfig.sauceRunnerConfigPath, "-s", options.SuiteName}, options.Environment)
}
