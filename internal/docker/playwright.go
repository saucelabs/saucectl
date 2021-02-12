package docker

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/framework"
	"path/filepath"

	"github.com/saucelabs/saucectl/internal/playwright"
)

// PlaywrightRunner represents the docker implementation of a test runner.
type PlaywrightRunner struct {
	ContainerRunner
	Project playwright.Project
}

// NewPlaywright creates a new PlaywrightRunner instance.
func NewPlaywright(c playwright.Project, imageLoc framework.ImageLocator) (*PlaywrightRunner, error) {
	r := PlaywrightRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			docker:          nil,
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.Kind,
				Version: c.Playwright.Version,
			},
			ImageLoc: imageLoc,
			ShowConsoleLog: c.ShowConsoleLog,
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

	if r.Project.Sauce.Concurrency > 1 {
		log.Info().Msg("concurrency > 1: file transfer mode forced to copy.")
		r.Project.Docker.FileTransfer = config.DockerFileCopy
	}

	if err := r.fetchImage(&r.Project.Docker); err != nil {
		return 1, err
	}

	containerOpts, results := r.createWorkerPool(r.Project.Sauce.Concurrency)
	defer close(results)

	go func() {
		for _, suite := range r.Project.Suites {
			containerOpts <- containerStartOptions{
				Docker:      r.Project.Docker,
				BeforeExec:  r.Project.BeforeExec,
				Project:     r.Project,
				SuiteName:   suite.Name,
				Environment: suite.Env,
				Files:       files,
			}
		}
		close(containerOpts)
	}()

	hasPassed := r.collectResults(results, len(r.Project.Suites))
	if !hasPassed {
		return 1, nil
	}
	return 0, nil
}
