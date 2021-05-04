package docker

import (
	"context"

	"github.com/saucelabs/saucectl/internal/artifact"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
)

// CypressRunner represents the docker implementation of a test runner.
type CypressRunner struct {
	ContainerRunner
	Project cypress.Project
}

// NewCypress creates a new CypressRunner instance.
func NewCypress(c cypress.Project, ms framework.MetadataService, wr job.Writer, rd job.Reader) (*CypressRunner, error) {
	r := CypressRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			docker:          nil,
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.Kind,
				Version: c.Cypress.Version,
			},
			FrameworkMeta:     ms,
			ShowConsoleLog:    c.ShowConsoleLog,
			JobWriter:         wr,
			JobReader:         rd,
			ArtfactDownloader: artifact.Downloader{JobReader: rd, Config: c.Artifacts.Download},
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
				Docker:         r.Project.Docker,
				BeforeExec:     r.Project.BeforeExec,
				Project:        r.Project,
				SuiteName:      suite.Name,
				Environment:    suite.Config.Env,
				RootDir:        r.Project.RootDir,
				Sauceignore:    r.Project.Sauce.Sauceignore,
				ConfigFilePath: r.Project.ConfigFilePath,
			}
		}
		close(containerOpts)
	}()

	hasPassed := r.collectResults(r.Project.Artifacts.Download, results, len(r.Project.Suites))
	if !hasPassed {
		return 1, nil
	}
	return 0, nil
}
