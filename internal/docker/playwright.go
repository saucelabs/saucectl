package docker

import (
	"context"

	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/notification/slack"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/report"
)

// PlaywrightRunner represents the docker implementation of a test runner.
type PlaywrightRunner struct {
	ContainerRunner
	Project playwright.Project
}

// NewPlaywright creates a new PlaywrightRunner instance.
func NewPlaywright(c playwright.Project, slSvc slack.Service, ms framework.MetadataService, wr job.Writer, jr job.Reader, dl download.ArtifactDownloader, reps []report.Reporter) (*PlaywrightRunner, error) {
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
			FrameworkMeta:     ms,
			ShowConsoleLog:    c.ShowConsoleLog,
			JobWriter:         wr,
			JobReader:         jr,
			ArtfactDownloader: dl,
			Reporters:         reps,
			SlackReporter: &slack.Reporter{
				Channels:    c.Notifications.Slack.Channels,
				Framework:   "playwright",
				Metadata:    c.Sauce.Metadata,
				TestEnv:     "docker",
				TestResults: []report.TestResult{},
				Config:      c.Notifications,
				Service:     slSvc,
			},
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
				Docker:         r.Project.Docker,
				BeforeExec:     r.Project.BeforeExec,
				Project:        r.Project,
				Browser:        suite.Params.BrowserName,
				DisplayName:    suite.Name,
				SuiteName:      suite.Name,
				Environment:    suite.Env,
				RootDir:        r.Project.RootDir,
				Sauceignore:    r.Project.Sauce.Sauceignore,
				ConfigFilePath: r.Project.ConfigFilePath,
				CLIFlags:       r.Project.CLIFlags,
				Timeout:        suite.Timeout,
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
