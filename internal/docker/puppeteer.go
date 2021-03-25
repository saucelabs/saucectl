package docker

import (
	"context"
	"github.com/saucelabs/saucectl/internal/puppeteer"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/framework"
)

// PuppeterRunner represents the docker implementation of a test runner.
type PuppeterRunner struct {
	ContainerRunner
	Project puppeteer.Project
}

// NewPuppeteer creates a new PuppeterRunner instance.
func NewPuppeteer(c puppeteer.Project, ms framework.MetadataService) (*PuppeterRunner, error) {
	r := PuppeterRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.Kind,
				Version: c.Puppeteer.Version,
			},
			FrameworkMeta:  ms,
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
func (r *PuppeterRunner) RunProject() (int, error) {
	if r.Project.Sauce.Concurrency > 1 {
		log.Info().Msg("concurrency > 1: forcing file transfer mode to use 'copy'.")
		r.Project.Docker.FileTransfer = config.DockerFileCopy
	}
	if err := r.fetchImage(&r.Project.Docker); err != nil {
		return 1, err
	}

	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

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
				Files:       []string{r.Project.RootDir},
				Sauceignore: r.Project.Sauce.Sauceignore,
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
