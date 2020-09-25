package docker

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/cli/streams"
	"github.com/saucelabs/saucectl/internal/fleet"
	"github.com/saucelabs/saucectl/internal/yaml"
	"github.com/saucelabs/saucectl/internal/utils"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
	"errors"
	"strings"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/progress"
)

// DefaultProjectPath represents the default project path. Test files will be located here.
const DefaultProjectPath = "/home/seluser"

// Runner represents the docker implementation of a test runner.
type Runner struct {
	runner.BaseRunner
	containerID string
	docker      *Handler
}

// NewRunner creates a new Runner instance.
func NewRunner(c config.Project, cli *command.SauceCtlCli, seq fleet.Sequencer) (*Runner, error) {
	progress.Show("Starting test runner for docker")
	defer progress.Stop()

	r := Runner{}
	r.Cli = cli
	r.Ctx = context.Background()
	r.Project = c
	r.Sequencer = seq

	var err error
	r.docker, err = Create()
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// RunProject runs the tests defined in config.Project.
func (r *Runner) RunProject() (int, error) {
	fid, err := fleet.Register(r.Ctx, r.Sequencer, r.Project.Files, r.Project.Suites)
	if err != nil {
		return 1, err
	}

	for _, suite := range r.Project.Suites {
		exitCode, err := r.runSuite(suite, fid)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}

	return 0, nil
}

// setup performs any necessary steps for a test runner to execute tests.
func (r *Runner) setup(suite config.Suite, run config.Run) error {
	err := r.docker.ValidateDependency()
	if err != nil {
		return fmt.Errorf("please verify that docker is installed and running: %v, "+
			" follow the guide at https://docs.docker.com/get-docker/", err)
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
	err = r.preExec(r.Project.Image.PreExec)
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

func (r* Runner) preExec(preExec string) (error) {
	tasks := utils.SplitLines(preExec)
	for _, task := range tasks {
		progress.Show("Running preExec task: %s", task)
		exitCode, err := r.execute(strings.Fields(task))
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return errors.New(fmt.Sprintf("Failed to run pre-exec task: %s", task))
		}
	}
	return nil
}

func (r *Runner) execute(cmd []string) (int, error) {
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
func (r *Runner) run() (int, error) {
	testCmd := []string{"npm", "test"}
	return r.execute(testCmd)
}

// teardown cleans up the test environment.
func (r *Runner) teardown(logDir string) error {
	for _, containerSrcPath := range runner.LogFiles {
		file := filepath.Base(containerSrcPath)
		hostDstPath := filepath.Join(logDir, file)
		if err := r.docker.CopyFromContainer(r.Ctx, r.containerID, containerSrcPath, hostDstPath); err != nil {
			continue
		}
	}

	if err := r.docker.ContainerStop(r.Ctx, r.containerID); err != nil {
		return err
	}

	if err := r.docker.ContainerRemove(r.Ctx, r.containerID); err != nil {
		return err
	}

	return nil
}

func (r *Runner) runSuite(suite config.Suite, fleetID string) (int, error) {
	for {
		next, err := r.Sequencer.NextAssignment(r.Ctx, fleetID, suite.Name)
		if err != nil {
			return 1, err
		}
		if next == "" {
			return 0, nil
		}

		run := config.Run{
			Match:       []string{next},
			ProjectPath: DefaultProjectPath,
		}

		code, err := r.runTest(suite, run)
		if err != nil || code != 0 {
			return code, err
		}
	}
}

func (r *Runner) runTest(suite config.Suite, run config.Run) (int, error) {
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err := r.teardown(r.Cli.LogDir); err != nil {
			log.Error().Err(err).Msg("Failed to tear down environment")
		}
	}()

	log.Info().Msg("Setting up test environment")
	if err := r.setup(suite, run); err != nil {
		return 1, err
	}

	log.Info().Msg("Starting tests")
	exitCode, err := r.run()
	if err != nil {
		return exitCode, err
	}

	log.Info().
		Int("ExitCode", exitCode).
		Msg("Command Finished")

	return exitCode, err
}
