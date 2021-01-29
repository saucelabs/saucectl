package docker

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/progress"
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

func (r *ContainerRunner) pullImage(img config.Image) error {
	// Check docker image name property from the config file.
	if img.Name == "" {
		return errors.New("no docker image specified")
	}

	// Check if image exists.
	baseImage := r.docker.GetImageFlavor(img)
	hasImage, err := r.docker.HasBaseImage(r.Ctx, baseImage)
	if err != nil {
		return err
	}

	// If it's our image, warn the user to not use the latest tag.
	if strings.Index(img.Name, "saucelabs") == 0 && img.Tag == "latest" {
		log.Warn().Msg("The use of 'latest' as the docker image tag is discouraged. " +
			"We recommend pinning the image to a specific version. " +
			"Please proceed with caution.")
	}

	// Only pull base image if not already installed.
	if !hasImage {
		progress.Show("Pulling image %s", baseImage)
		defer progress.Stop()
		if err := r.docker.PullBaseImage(r.Ctx, img); err != nil {
			return err
		}
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
