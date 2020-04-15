package runner

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/docker"
)

type localRunner struct {
	baseRunner
	containerID string
	docker      *docker.Handler
}

func newLocalRunner(c config.JobConfiguration, cli *command.SauceCtlCli) (localRunner, error) {
	runner := localRunner{}
	ctx := context.Background()
	dockerClient, err := docker.Create()
	if err != nil {
		return runner, err
	}

	err = dockerClient.ValidateDependency()
	if err != nil {
		return runner, errors.New("Docker is not installed")
	}

	hasBaseImage, err := dockerClient.HasBaseImage(ctx, c.Image.Base)
	if err != nil {
		return runner, err
	}

	if !hasBaseImage {
		if err := dockerClient.PullBaseImage(ctx, c.Image.Base); err != nil {
			return runner, err
		}
	}

	container, err := dockerClient.StartContainer(ctx, c.Image.Base)
	if err != nil {
		return runner, err
	}

	// wait until Xvfb started
	// ToDo(Christian): make this dynamic
	time.Sleep(1 * time.Second)

	// get runner config
	tmpDir, err := ioutil.TempDir("", "saucectl")
	if err != nil {
		return runner, err
	}
	defer os.RemoveAll(tmpDir)
	hostDstPath := filepath.Join(tmpDir, filepath.Base(runnerConfigPath))
	if err := dockerClient.CopyFromContainer(ctx, container.ID, runnerConfigPath, hostDstPath); err != nil {
		return runner, err
	}

	rc, err := config.NewRunnerConfiguration(hostDstPath)
	if err != nil {
		return runner, err
	}

	runner.cli = cli
	runner.context = ctx
	runner.jobConfig = c
	runner.runnerConfig = rc
	runner.startTime = makeTimestamp()
	runner.docker = dockerClient
	runner.containerID = container.ID
	return runner, nil
}

func (r localRunner) Setup() error {
	if err := r.docker.CopyTestFilesToContainer(r.context, r.containerID, r.jobConfig.Files, r.runnerConfig.TargetDir); err != nil {
		return err
	}
	return nil
}

func (r localRunner) Run() (int, error) {
	if r.containerID == "" {
		return 1, errors.New("No container id found, run testrunner setup first")
	}

	var (
		out, stderr io.Writer
		in          io.ReadCloser
	)
	out = r.cli.Out()
	stderr = r.cli.Out()

	if err := r.cli.In().CheckTty(false, true); err != nil {
		return 1, err
	}

	createResp, attachResp, err := r.docker.ExecuteTest(r.context, r.containerID)
	if err != nil {
		return 1, err
	}

	defer attachResp.Close()

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		errCh <- func() error {
			streamer := ioStreamer{
				streams:      r.cli,
				inputStream:  in,
				outputStream: out,
				errorStream:  stderr,
				resp:         *attachResp,
				detachKeys:   "",
			}

			return streamer.stream(r.context)
		}()
	}()

	if err := <-errCh; err != nil {
		return 1, err
	}

	exitCode, err := r.docker.ExecuteInspect(r.context, createResp.ID)
	if err != nil {
		return 1, err
	}

	return exitCode, nil
}

func (r localRunner) Teardown(logDir string) error {
	for _, containerSrcPath := range logFiles {
		file := filepath.Base(containerSrcPath)
		hostDstPath := filepath.Join(logDir, file)
		if err := r.docker.CopyFromContainer(r.context, r.containerID, containerSrcPath, hostDstPath); err != nil {
			continue
		}
	}

	if err := r.docker.ContainerStop(r.context, r.containerID); err != nil {
		return err
	}

	if err := r.docker.ContainerRemove(r.context, r.containerID); err != nil {
		return err
	}

	return nil
}
