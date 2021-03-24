package docker

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/testcafe"
)

// TestcafeRunner represents the docker implementation of a test runner.
type TestcafeRunner struct {
	ContainerRunner
	Project testcafe.Project
}

// NewTestcafe creates a new TestcafeRunner instance.
func NewTestcafe(c testcafe.Project, ms framework.MetadataService) (*TestcafeRunner, error) {
	r := TestcafeRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.Kind,
				Version: c.Testcafe.Version,
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
func (r *TestcafeRunner) RunProject() (int, error) {
	if r.Project.Sauce.Concurrency > 1 {
		log.Info().Msg("concurrency > 1: forcing file transfer mode to use 'copy'.")
		r.Project.Docker.FileTransfer = config.DockerFileCopy
	}
	if err := r.fetchImage(&r.Project.Docker); err != nil {
		return 1, err
	}

	skipSuites := false
	sigChan := registerSkipSuiteOnSignal(&skipSuites)
	defer unregisterSignalCapture(sigChan)

	containerOpts, results := r.createWorkerPool(r.Project.Sauce.Concurrency, &skipSuites)
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
