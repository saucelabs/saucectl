package legacydocker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fleet"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/streams"
	"github.com/saucelabs/saucectl/internal/yaml"
)

// DefaultProjectPath represents the default project path. Test files will be located here.
const DefaultProjectPath = "/home/seluser"

// LegacyRunner represents the docker implementation of a test runner. Only meant for use with frameworks that did not
// yet support native configs.
//
// Deprecated: Use the appropriate, framework specific runner instead.
type LegacyRunner struct {
	runner.BaseRunner
	containerID string
	docker      *LegacyHandler
}

// NewRunner creates a new LegacyRunner instance.
//
// Deprecated: Use the appropriate, framework specific runner instead.
func NewRunner(c config.Project, cli *command.SauceCtlCli, seq fleet.Sequencer) (*LegacyRunner, error) {
	progress.Show("Starting test runner for docker")
	defer progress.Stop()

	r := LegacyRunner{}
	r.Cli = cli
	r.Ctx = context.Background()
	r.Project = c
	r.Sequencer = seq

	var err error
	r.docker, err = CreateLegacy()
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// RunProject runs the tests defined in config.Project.
func (r *LegacyRunner) RunProject() (int, error) {
	fid, err := fleet.Register(r.Ctx, r.Sequencer, "", r.Project.Files, r.Project.Suites)
	if err != nil {
		return 1, err
	}

	errorCount := 0
	for _, suite := range r.Project.Suites {
		err = r.runSuite(suite, fid)
		if err != nil {
			errorCount++
		}
	}
	if errorCount > 0 {
		log.Error().Msgf("%d suite(s) failed", errorCount)
	}
	return errorCount, nil
}

// setup performs any necessary steps for a test runner to execute tests.
func (r *LegacyRunner) setup(suite config.Suite, run config.Run) error {
	err := r.docker.ValidateDependency()
	if err != nil {
		return fmt.Errorf("please verify that docker is installed and running: %v, "+
			" follow the guide at https://docs.docker.com/get-docker/", err)
	}

	// check image base property from the config file
	if r.Project.Image.Base == "" {
		return errors.New("no docker image specified")
	}

	// check if image is existing
	baseImage := r.docker.GetImageFlavor(r.Project)
	hasImage, err := r.docker.HasBaseImage(r.Ctx, baseImage)
	if err != nil {
		return err
	}

	// only pull base image if not already installed
	progress.Show("Pulling test runner image %s", baseImage)
	defer progress.Stop()

	if !hasImage {
		if err := r.docker.PullBaseImage(r.Ctx, r.Project); err != nil {
			return err
		}
	}

	progress.Show("Starting container %s", baseImage)
	container, err := r.docker.StartContainer(r.Ctx, r.Project, suite)
	if err != nil {
		return err
	}
	r.containerID = container.ID

	progress.Show("Preparing container")
	// TODO replace sleep with actual checks & confirmation
	// wait until Xvfb started
	time.Sleep(1 * time.Second)

	// get runner config
	tmpDir, err := ioutil.TempDir("", "saucectl")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	hostDstPath := filepath.Join(tmpDir, filepath.Base(runner.ConfigPath))
	if err := r.docker.CopyFromContainer(r.Ctx, container.ID, runner.ConfigPath, hostDstPath); err != nil {
		return err
	}

	r.RunnerConfig, err = config.NewRunnerConfiguration(hostDstPath)
	if err != nil {
		return err
	}

	progress.Show("Setting up test files for container")
	rcPath, err := yaml.TempFile("run.yaml", run)
	if err != nil {
		return err
	}
	if err := r.docker.CopyToContainer(r.Ctx, r.containerID, rcPath, r.RunnerConfig.RootDir); err != nil {
		return err
	}

	// running pre-exec tasks
	err = r.beforeExec(r.Project.BeforeExec)
	if err != nil {
		return err
	}
	// start port forwarding
	sockatCmd := []string{
		"socat",
		"tcp-listen:9222,reuseaddr,fork",
		"tcp:localhost:9223",
	}

	if _, _, err := r.docker.Execute(r.Ctx, r.containerID, sockatCmd); err != nil {
		return err
	}

	return nil
}

func (r *LegacyRunner) beforeExec(tasks []string) error {
	for _, task := range tasks {
		progress.Show("Running BeforeExec task: %s", task)
		exitCode, err := r.execute(strings.Fields(task))
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return fmt.Errorf("failed to run BeforeExec task: %s - exit code %d", task, exitCode)
		}
	}
	return nil
}

func (r *LegacyRunner) execute(cmd []string) (int, error) {
	var (
		out, stderr io.Writer
		in          io.ReadCloser
	)
	out = r.Cli.Out()
	stderr = r.Cli.Out()

	if err := r.Cli.In().CheckTty(false, true); err != nil {
		return 1, err
	}
	createResp, attachResp, err := r.docker.Execute(r.Ctx, r.containerID, cmd)
	if err != nil {
		return 1, err
	}
	defer attachResp.Close()
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		errCh <- func() error {
			streamer := streams.IOStreamer{
				Streams:      r.Cli,
				InputStream:  in,
				OutputStream: out,
				ErrorStream:  stderr,
				Resp:         *attachResp,
			}

			return streamer.Stream(r.Ctx)
		}()
	}()

	if err := <-errCh; err != nil {
		return 1, err
	}

	exitCode, err := r.docker.ExecuteInspect(r.Ctx, createResp.ID)
	if err != nil {
		return 1, err
	}
	return exitCode, nil

}

// run runs the tests defined in the config.Project.
func (r *LegacyRunner) run() (int, error) {
	return r.execute([]string{"npm", "test"})
}

// teardown cleans up the test environment.
func (r *LegacyRunner) teardown(logDir string) error {
	for _, containerSrcPath := range runner.LogFiles {
		file := filepath.Base(containerSrcPath)
		hostDstPath := filepath.Join(logDir, file)
		if err := r.docker.CopyFromContainer(r.Ctx, r.containerID, containerSrcPath, hostDstPath); err != nil {
			continue
		}
	}

	// checks that container exists before stopping and removing it
	if _, err := r.docker.ContainerInspect(r.Ctx, r.containerID); err != nil {
		return err
	}

	if err := r.docker.ContainerStop(r.Ctx, r.containerID); err != nil {
		return err
	}

	if err := r.docker.ContainerRemove(r.Ctx, r.containerID); err != nil {
		return err
	}

	return nil
}

func (r *LegacyRunner) runSuite(suite config.Suite, fleetID string) error {
	var assignments []string
	for {
		next, err := r.Sequencer.NextAssignment(r.Ctx, fleetID, suite.Name)
		if err != nil {
			return err
		}
		if next == "" {
			break
		}
		assignments = append(assignments, next)
	}

	if len(assignments) == 0 {
		log.Info().Msg("No tests detected. Skipping suite.")
		return nil
	}

	run := config.Run{
		Match:       assignments,
		ProjectPath: DefaultProjectPath,
	}
	return r.runTest(suite, run)
}

func (r *LegacyRunner) runTest(suite config.Suite, run config.Run) error {
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err := r.teardown(r.Cli.LogDir); err != nil {
			if !r.docker.IsErrNotFound(err) {
				log.Error().Err(err).Msg("Failed to tear down environment")
			}
		}
	}()

	log.Info().Msg("Setting up test environment")
	if err := r.setup(suite, run); err != nil {
		log.Err(err).Msg("Failed to setup test environment")
		return err
	}

	log.Info().Msg("Starting tests")
	exitCode, err := r.run()
	log.Info().
		Int("ExitCode", exitCode).
		Msg("Command Finished")

	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("exitCode is %d", exitCode)
	}
	return nil
}
