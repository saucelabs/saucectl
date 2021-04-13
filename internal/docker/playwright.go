package docker

import (
	"context"

	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/playwright"
)

// PlaywrightRunner represents the docker implementation of a test runner.
type PlaywrightRunner struct {
	ContainerRunner
	Project playwright.Project
}

// NewPlaywright creates a new PlaywrightRunner instance.
func NewPlaywright(c playwright.Project, ms framework.MetadataService, wr job.Writer) (*PlaywrightRunner, error) {
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
			FrameworkMeta:  ms,
			ShowConsoleLog: c.ShowConsoleLog,
			JobWriter:      wr,
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
	verifyFileTransferCompatibility(r.Project.Sauce.Concurrency, &r.Project.Docker)

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
				RootDir:     r.Project.RootDir,
				Sauceignore: r.Project.Sauce.Sauceignore,
				RawConfig:   r.Project.RawConfig,
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
