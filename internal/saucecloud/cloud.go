package saucecloud

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/progress"
	"github.com/saucelabs/saucectl/internal/archive/zip"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/jsonio"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/storage"
	"path/filepath"
)

// CloudRunner represents the cloud runner for the Sauce Labs cloud.
type CloudRunner struct {
	ProjectUploader storage.ProjectUploader
	JobStarter      job.Starter
	JobReader       job.Reader
	CCYReader       concurrency.Reader
	Region          region.Region
	ShowConsoleLog  bool
}

func (r *CloudRunner) archiveProject(project interface{}, tempDir string, files []string) (string, error) {
	zipName := filepath.Join(tempDir, "app.zip")
	z, err := zip.NewWriter(zipName)
	if err != nil {
		return "", err
	}
	defer z.Close()

	rcPath := filepath.Join(tempDir, "sauce-runner.json")
	if err := jsonio.WriteFile(rcPath, project); err != nil {
		return "", err
	}
	files = append(files, rcPath)

	for _, f := range files {
		if err := z.Add(f, ""); err != nil {
			return "", err
		}
	}

	return zipName, z.Close()
}

func (r *CloudRunner) uploadProject(filename string) (string, error) {
	progress.Show("Uploading project")
	resp, err := r.ProjectUploader.Upload(filename)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Str("storageId", resp.ID).Msg("Project uploaded.")
	return resp.ID, nil
}

// logSuite display the result of a suite
func (r *CloudRunner) logSuite(res result) {
	if res.job.ID == "" {
		log.Error().Str("suite", res.suiteName).Msgf("failed to start")
		log.Error().Str("suite", res.suiteName).Msgf("%s", res.err)
		return
	}
	resultStr := "Passed"
	if !res.job.Passed {
		resultStr = "Failed"
	}
	jobDetailsPage := fmt.Sprintf("%s/tests/%s", r.Region.AppBaseURL(), res.job.ID)
	log.Info().Str("suite", res.suiteName).Msgf("Status: %s - %s", resultStr, jobDetailsPage)
	r.logSuiteConsole(res)
}

// logSuiteError display the console output when tests from a suite are failing
func (r *CloudRunner) logSuiteConsole(res result) {
	// To avoid clutter, we don't show the console on job passes.
	if res.job.Passed || !r.ShowConsoleLog {
		return
	}

	// Display log only when at least it has started
	assetContent, err := r.JobReader.GetJobAssetFileContent(context.Background(), res.job.ID, resto.ConsoleLogAsset)
	if err != nil {
		log.Warn().Str("suite", res.suiteName).Msg("Failed to get job asset.")
	} else {
		log.Info().Msg(fmt.Sprintf("Test %s %s", res.job.ID, resto.ConsoleLogAsset))
		log.Info().Msg(string(assetContent))
	}
}
