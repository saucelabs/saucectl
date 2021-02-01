package docker

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/config"
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
	containerID     string
	docker          *Handler
	containerConfig *containerConfig
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
	if strings.Index(img, "saucelabs") == 0 && strings.Contains(img, "latest") {
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

// setup performs any necessary steps for a test runner to execute tests.
func (r *ContainerRunner) setupImage(confd config.Docker, beforeExec []string, project interface{}, files []string) error {
	if !r.docker.IsInstalled() {
		return fmt.Errorf("please verify that docker is installed and running: " +
			" follow the guide at https://docs.docker.com/get-docker/")
	}

	if err := r.pullImage(confd.Image); err != nil {
		return err
	}

	container, err := r.docker.StartContainer(r.Ctx, files, confd)
	if err != nil {
		return err
	}
	r.containerID = container.ID

	pDir, err := r.docker.ProjectDir(r.Ctx, confd.Image)
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("", "saucectl")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	rcPath := filepath.Join(tmpDir, SauceRunnerConfigFile)
	if err := jsonio.WriteFile(rcPath, project); err != nil {
		return err
	}

	if err := r.docker.CopyToContainer(r.Ctx, r.containerID, rcPath, pDir); err != nil {
		return err
	}
	r.containerConfig.sauceRunnerConfigPath = path.Join(pDir, SauceRunnerConfigFile)

	// running pre-exec tasks
	err = r.beforeExec(beforeExec)
	if err != nil {
		return err
	}

	return nil
}

func (r *ContainerRunner) run(cmd []string, env map[string]string) error {
	defer func() {
		log.Info().Msg("Tearing down environment")
		if err := r.docker.Teardown(r.Ctx, r.containerID); err != nil {
			if !r.docker.IsErrNotFound(err) {
				log.Error().Err(err).Msg("Failed to tear down environment")
			}
		}
	}()

	exitCode, err := r.docker.ExecuteAttach(r.Ctx, r.containerID, r.Cli, cmd, env)
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

func (r *ContainerRunner) beforeExec(tasks []string) error {
	for _, task := range tasks {
		log.Info().Str("task", task).Msg("Running BeforeExec")
		exitCode, err := r.docker.ExecuteAttach(r.Ctx, r.containerID, r.Cli, strings.Fields(task), nil)
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return fmt.Errorf("failed to run BeforeExec task: %s - exit code %d", task, exitCode)
		}
	}
	return nil
}
