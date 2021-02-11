package docker

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/framework"
)

// CypressRunner represents the docker implementation of a test runner.
type CypressRunner struct {
	ContainerRunner
	Project cypress.Project
}

// NewCypress creates a new CypressRunner instance.
func NewCypress(c cypress.Project, cli *command.SauceCtlCli, imageLoc framework.ImageLocator) (*CypressRunner, error) {
	r := CypressRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			Cli:             cli,
			containerID:     "",
			docker:          nil,
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.Kind,
				Version: c.Cypress.Version,
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
func (r *CypressRunner) RunProject() (int, error) {
	files := []string{
		r.Project.Cypress.ConfigFile,
		r.Project.Cypress.ProjectPath,
	}

	if r.Project.Cypress.EnvFile != "" {
		files = append(files, r.Project.Cypress.EnvFile)
	}

	errorCount := 0
	for _, suite := range r.Project.Suites {
		err := r.RunSuite(ContainerStartOptions{
			Docker:      r.Project.Docker,
			BeforeExec:  r.Project.BeforeExec,
			Project:     r.Project,
			SuiteName:   suite.Name,
			Environment: suite.Config.Env,
			Files:       files,
		})

		if err != nil {
			errorCount++
		}
	}
	if errorCount > 0 {
		log.Error().Msgf("%d suite(s) failed", errorCount)
	}
	return errorCount, nil
}
