package docker

import (
	"context"

	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/report"
)

// CypressRunner represents the docker implementation of a test runner.
type CypressRunner struct {
	ContainerRunner
	Project cypress.Project
}

// NewCypress creates a new CypressRunner instance.
func NewCypress(c cypress.Project, ms framework.MetadataService, wr job.Writer, jr job.Reader, dl job.ArtifactDownloader, reps []report.Reporter) (*CypressRunner, error) {
	r := CypressRunner{
		Project: c,
		ContainerRunner: ContainerRunner{
			Ctx:             context.Background(),
			docker:          nil,
			containerConfig: &containerConfig{},
			Framework: framework.Framework{
				Name:    c.GetKind(),
				Version: c.GetVersion(),
			},
			FrameworkMeta:          ms,
			ShowConsoleLog:         c.GetShowConsoleLog(),
			JobWriter:              wr,
			JobReader:              jr,
			ArtfactDownloader:      dl,
			Reporters:              reps,
			MetadataSearchStrategy: framework.NewSearchStrategy(c.GetVersion(), c.GetRootDir()),
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
	docker := r.Project.GetDocker()
	verifyFileTransferCompatibility(r.Project.GetSauceCfg().Concurrency, &docker)

	if err := r.fetchImage(&docker); err != nil {
		return 1, err
	}

	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	containerOpts, results := r.createWorkerPool(r.Project.GetSauceCfg().Concurrency)
	defer close(results)

	go func() {
		for _, suite := range r.Project.GetSuites() {
			containerOpts <- containerStartOptions{
				Docker:         docker,
				BeforeExec:     r.Project.GetBeforeExec(),
				Project:        r.Project,
				Browser:        suite.Browser,
				DisplayName:    suite.Name,
				SuiteName:      suite.Name,
				Environment:    suite.Env,
				RootDir:        r.Project.GetRootDir(),
				Sauceignore:    r.Project.GetSauceCfg().Sauceignore,
				ConfigFilePath: r.Project.GetCfgPath(),
				CLIFlags:       r.Project.GetCLIFlags(),
				Timeout:        suite.Timeout,
			}
		}
		close(containerOpts)

	}()

	hasPassed := r.collectResults(r.Project.GetArtifactsCfg().Download, results, r.Project.GetSuiteCount())
	if !hasPassed {
		return 1, nil
	}
	return 0, nil
}
