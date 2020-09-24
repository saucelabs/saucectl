package docker

import (
	"context"
	"fmt"
	"github.com/saucelabs/saucectl/cli/runner"
	"github.com/saucelabs/saucectl/cli/streams"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/yaml"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
	"strings"
	"errors"

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
	tmpDir      string
}

// NewRunner creates a new Runner instance.
func NewRunner(c config.Project, s config.Suite, cli *command.SauceCtlCli) (*Runner, error) {
	progress.Show("Starting test runner for docker")
	defer progress.Stop()

	r := Runner{}
	r.Cli = cli
	r.Ctx = context.Background()
	r.Project = c
	r.Suite = s

	var err error
	r.docker, err = Create()
	if err != nil {
		return nil, err
	}

	r.tmpDir, err = ioutil.TempDir("", "saucectl")
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// Setup performs any necessary steps for a test runner to execute tests.
func (r *Runner) Setup() error {
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
	container, err := r.docker.StartContainer(r.Ctx, r.Project, r.Suite)
	if err != nil {
		return err
	}
	r.containerID = container.ID

	progress.Show("Preparing container")
	// TODO replace sleep with actual checks & confirmation
	// wait until Xvfb started
	time.Sleep(1 * time.Second)

	// get runner config
	defer os.RemoveAll(r.tmpDir)
	hostDstPath := filepath.Join(r.tmpDir, filepath.Base(runner.ConfigPath))
	if err := r.docker.CopyFromContainer(r.Ctx, container.ID, runner.ConfigPath, hostDstPath); err != nil {
		return err
	}

	r.RunnerConfig, err = config.NewRunnerConfiguration(hostDstPath)
	if err != nil {
		return err
	}

	progress.Show("Setting up test files for container")
	rc := config.Run{
		ProjectPath: DefaultProjectPath,
	}

	files, err := fpath.Walk(r.Project.Files, r.Suite.Match)
	if err != nil {
		return err
	}
	rc.Match = files

	rcPath, err := yaml.TempFile("run.yaml", rc)
	if err != nil {
		return err
	}
	if err := r.docker.CopyToContainer(r.Ctx, r.containerID, rcPath, r.RunnerConfig.RootDir); err != nil {
		return err
	}

	// running pre-exec tasks
	err = r.PreExec(r.Project.Image.PreExec)
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

func (r* Runner) PreExec(preExec string) (error) {
	tasks := yaml.SplitLines(preExec)
	for _, task := range tasks {
		progress.Show("Running preExec task: %s", task)
		exitCode, err := r.Execute(strings.Fields(task))
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return errors.New(fmt.Sprintf("Failed to run pre-exec task: %s", task))
		}
	}
	return nil
}

func (r* Runner) Execute(cmd []string) (int, error) {
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
// Run runs the tests defined in the config.Project.
func (r *Runner) Run() (int, error) {
	testCmd := []string{"npm", "test"}
	return r.Execute(testCmd)
}

// Teardown cleans up the test environment.
func (r *Runner) Teardown(logDir string) error {
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
