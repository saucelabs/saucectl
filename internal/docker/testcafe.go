package docker

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/testcafe"
)

// TestcafeRunner represents the docker implementation of a test runner.
type TestcafeRunner struct {
	ContainerRunner
	Project testcafe.Project
}

// NewTestcafe creates a new TestcafeRunner instance.
func NewTestcafe(c testcafe.Project, cli *command.SauceCtlCli, imageLoc framework.ImageLocator) (*TestcafeRunner, error) {
	r := TestcafeRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			Cli:             cli,
			containerID:     "",
			docker:          nil,
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.Kind,
				Version: c.Testcafe.Version,
			},
			ImageLoc: imageLoc,
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
func (r *TestcafeRunner) RunProject() (int, error) {
	errCnt := 0
	for _, suite := range r.Project.Suites {
		log.Info().Msg("Setting up test enviornment")
		if err := r.setupImage(r.Project.Docker, r.Project.BeforeExec, r.Project, []string{r.Project.Testcafe.ProjectPath}); err != nil {
			log.Err(err).Msg("Failed to setup test environment")
			return 1, err
		}

		err := r.run([]string{"npm", "test", "--", "-r", r.containerConfig.sauceRunnerConfigPath, "-s", suite.Name}, suite.Env)
		if err != nil {
			errCnt++
		}
	}
	if errCnt > 0 {
		log.Error().Msgf("%d suite(s) failed", errCnt)
	}
	return errCnt, nil
}
