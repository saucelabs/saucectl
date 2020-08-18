package runner

import (
	"context"
	"fmt"
	"github.com/saucelabs/saucectl/cli/streams"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/docker"
	"github.com/saucelabs/saucectl/cli/progress"
)

type DockerRunner struct {
	BaseRunner
	containerID string
	docker      *docker.Handler
	tmpDir      string
}

func NewDockerRunner(c config.Project, cli *command.SauceCtlCli) (*DockerRunner, error) {
	progress.Show("Starting test runner for docker")
	defer progress.Stop()

	runner := DockerRunner{}
	runner.Cli = cli
	runner.Ctx = context.Background()
	runner.Project = c
	runner.startTime = makeTimestamp()

	var err error
	runner.docker, err = docker.Create()
	if err != nil {
		return nil, err
	}

	runner.tmpDir, err = ioutil.TempDir("", "saucectl")
	if err != nil {
		return nil, err
	}

	return &runner, nil
}

func (r *DockerRunner) Setup() error {
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
	container, err := r.docker.StartContainer(r.Ctx, r.Project)
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
	hostDstPath := filepath.Join(r.tmpDir, filepath.Base(RunnerConfigPath))
	if err := r.docker.CopyFromContainer(r.Ctx, container.ID, RunnerConfigPath, hostDstPath); err != nil {
		return err
	}

	r.RunnerConfig, err = config.NewRunnerConfiguration(hostDstPath)
	if err != nil {
		return err
	}

	progress.Show("Copying test files to container")
	if err := r.docker.CopyTestFilesToContainer(r.Ctx, r.containerID, r.Project.Files, r.RunnerConfig.TargetDir); err != nil {
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

func (r *DockerRunner) Run() (int, error) {
	var (
		out, stderr io.Writer
		in          io.ReadCloser
	)
	out = r.Cli.Out()
	stderr = r.Cli.Out()

	if err := r.Cli.In().CheckTty(false, true); err != nil {
		return 1, err
	}

	/*
		Want to improve this, disabling it for a bit
		exec := r.Project.Image.Exec
		testCmd := strings.Split(exec, " ")
	*/
	testCmd := []string{"npm", "test"}
	createResp, attachResp, err := r.docker.Execute(r.Ctx, r.containerID, testCmd)
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

func (r *DockerRunner) Teardown(logDir string) error {
	for _, containerSrcPath := range LogFiles {
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
