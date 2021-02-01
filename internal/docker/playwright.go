package docker

import (
	"context"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/playwright"
)

// PlaywrightRunner represents the docker implementation of a test runner.
type PlaywrightRunner struct {
	ContainerRunner
	Project playwright.Project
}

// NewPlaywright creates a new PlaywrightRunner instance.
func NewPlaywright(c playwright.Project, cli *command.SauceCtlCli) (*PlaywrightRunner, error) {
	r := PlaywrightRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			Cli:             cli,
			containerID:     "",
			docker:          nil,
			containerConfig: &containerConfig{},
		},
	}

	var err error
	r.docker, err = Create()
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// RunProject runs the tests defined in config.Project.
func (r *PlaywrightRunner) RunProject() (int, error) {
	files := []string{
		r.Project.Playwright.LocalProjectPath,
	}
	r.Project.Playwright.ProjectPath = filepath.Base(r.Project.Playwright.ProjectPath)

	errorCount := 0
	for _, suite := range r.Project.Suites {
		log.Info().Msg("Setting up test environment")
		if err := r.setupImage(r.Project.Docker, r.Project.BeforeExec, r.Project, files); err != nil {
			log.Err(err).Msg("Failed to setup test environment")
			return 1, err
		}

		err := r.run([]string{"npm", "test", "--", "-r", r.containerConfig.sauceRunnerConfigPath, "-s", suite.Name},
			suite.Env)
		if err != nil {
			errorCount++
		}
	}
	if errorCount > 0 {
		log.Error().Msgf("%d suite(s) failed", errorCount)
	}
	return errorCount, nil
}
