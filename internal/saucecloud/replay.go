package saucecloud

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/puppeteer/replay"
)

// ReplayRunner represents the Sauce Labs cloud implementation for puppeteer-replay.
type ReplayRunner struct {
	CloudRunner
	Project replay.Project
}

// RunProject runs the tests defined in cypress.Project.
func (r *ReplayRunner) RunProject() (int, error) {
	exitCode := 1

	if err := r.validateTunnel(r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner); err != nil {
		return 1, err
	}

	var files []string
	var suiteNames []string
	for _, suite := range r.Project.Suites {
		suiteNames = append(suiteNames, suite.Name)
		files = append(files, suite.Recording)
	}

	if r.Project.DryRun {
		log.Warn().Msg("Running tests in dry run mode.")
		tmpDir, err := os.MkdirTemp("./", "sauce-app-payload-*")
		if err != nil {
			return 1, err
		}
		log.Info().Msgf("The following test suites would have run: [%s].", suiteNames)
		zipName, err := r.archiveFiles(r.Project, tmpDir, files, "")
		if err != nil {
			return 1, err
		}

		log.Info().Msgf("Saving bundled project to %s.", zipName)
		return 0, nil
	}

	fileURI, err := r.remoteArchiveFiles(r.Project, files, "")
	if err != nil {
		return exitCode, err
	}

	passed := r.runSuites(fileURI)
	if passed {
		exitCode = 0
	}

	return exitCode, nil
}

func (r *ReplayRunner) runSuites(fileURI string) bool {
	sigChan := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(sigChan)

	jobOpts, results, err := r.createWorkerPool(r.Project.Sauce.Concurrency, r.Project.Sauce.Retries)
	if err != nil {
		return false
	}
	defer close(results)

	// Submit suites to work on.
	go func() {
		for _, s := range r.Project.Suites {
			jobOpts <- job.StartOptions{
				ConfigFilePath:   r.Project.ConfigFilePath,
				DisplayName:      s.Name,
				Timeout:          s.Timeout,
				App:              fileURI,
				Suite:            s.Name,
				Framework:        "puppeteer-replay",
				FrameworkVersion: "latest",
				BrowserName:      s.BrowserName,
				BrowserVersion:   s.BrowserVersion,
				PlatformName:     s.Platform,
				Name:             s.Name,
				Build:            r.Project.Sauce.Metadata.Build,
				Tags:             r.Project.Sauce.Metadata.Tags,
				Tunnel: job.TunnelOptions{
					ID:     r.Project.Sauce.Tunnel.Name,
					Parent: r.Project.Sauce.Tunnel.Owner,
				},
				Experiments: r.Project.Sauce.Experiments,
				Attempt:     0,
				Retries:     r.Project.Sauce.Retries,
				Timezone:    s.Timezone,
			}
		}
	}()

	return r.collectResults(r.Project.Artifacts.Download, results, len(r.Project.Suites))
}
