package docker

import (
	"context"

	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/puppeteer"
)

// PuppeterRunner represents the docker implementation of a test runner.
type PuppeterRunner struct {
	ContainerRunner
	Project puppeteer.Project
}

// NewPuppeteer creates a new PuppeterRunner instance.
func NewPuppeteer(c puppeteer.Project, ms framework.MetadataService, wr job.Writer, dl download.ArtifactDownloader) (*PuppeterRunner, error) {
	r := PuppeterRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.Kind,
				Version: c.Puppeteer.Version,
			},
			FrameworkMeta:     ms,
			ShowConsoleLog:    c.ShowConsoleLog,
			JobWriter:         wr,
			ArtfactDownloader: dl,
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
				Browser:        suite.Browser,
				SuiteName:      suite.Name,
				Environment:    suite.Env,
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
