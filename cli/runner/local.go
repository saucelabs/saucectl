package runner

import (
	"errors"
	"io"
	"path/filepath"
	"time"
)

type localRunner struct {
	baseRunner
	containerID string
}

func (r localRunner) Setup() error {
	hasBaseImage, err := r.cli.Docker.HasBaseImage(r.context, r.config.Image.Base)
	if err != nil {
		return err
	}

	if !hasBaseImage {
		if err := r.cli.Docker.PullBaseImage(r.context, r.config.Image.Base); err != nil {
			return err
		}
	}

	container, err := r.cli.Docker.StartContainer(r.context, r.config.Image.Base)
	if err != nil {
		return err
	}

	r.containerID = container.ID

	// wait until Xvfb started
	// ToDo(Christian): make this dynamic
	time.Sleep(1 * time.Second)

	if err := r.cli.Docker.CopyTestFilesToContainer(r.context, container.ID, r.config.Files, targetDir); err != nil {
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

	createResp, attachResp, err := r.cli.Docker.ExecuteTest(r.context, r.containerID)
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

	exitCode, err := r.cli.Docker.ExecuteInspect(r.context, createResp.ID)
	if err != nil {
		return 1, err
	}

	return exitCode, nil
}

func (r localRunner) Teardown(logDir string) error {
	for _, containerSrcPath := range logFiles {
		file := filepath.Base(containerSrcPath)
		hostDstPath := filepath.Join(logDir, file)
		if err := r.cli.Docker.CopyFromContainer(r.context, r.containerID, containerSrcPath, hostDstPath); err != nil {
			continue
		}
	}

	if err := r.cli.Docker.ContainerStop(r.context, r.containerID); err != nil {
		return err
	}

	if err := r.cli.Docker.ContainerRemove(r.context, r.containerID); err != nil {
		return err
	}

	return nil
}
