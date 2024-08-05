package retry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/saucecloud/zip"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/saucelabs/saucectl/internal/storage"
)

type SauceReportRetrier struct {
	VDCReader       job.Reader
	ProjectUploader storage.AppService

	Project Project
}

func (r *SauceReportRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if r.VDCReader != nil && opt.SmartRetry.FailedOnly {
		r.RetryFailedTests(&opt, previous)
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}

func (r *SauceReportRetrier) RetryFailedTests(opt *job.StartOptions, previous job.Job) {
	if previous.Status == job.StateError {
		log.Warn().Msg(msg.UnreliableReport)
		log.Info().Msg(msg.SkippingSmartRetries)
		return
	}

	report, err := r.getSauceReport(previous)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToFetchFile, saucereport.SauceReportFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		return
	}
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if err != nil {
		log.Err(err).Msg(msg.UnableToCreateRunnerConfig)
		log.Info().Msg(msg.SkippingSmartRetries)
		return
	}

	if err := r.Project.FilterFailedTests(opt.Name, report); err != nil {
		log.Err(err).Msg(msg.UnableToFilterFailedTests)
		log.Info().Msg(msg.SkippingSmartRetries)
		return
	}

	runnerFile, err := zip.ArchiveRunnerConfig(r.Project, tempDir)
	if err != nil {
		log.Err(err).Msg(msg.UnableToArchiveRunnerConfig)
		log.Info().Msg(msg.SkippingSmartRetries)
		return
	}

	fileURL, err := r.uploadConfig(runnerFile)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToUploadConfig, runnerFile)
		log.Info().Msg(msg.SkippingSmartRetries)
		return
	}

	if len(opt.OtherApps) == 0 {
		opt.OtherApps = []string{fmt.Sprintf("storage:%s", fileURL)}
	} else {
		opt.OtherApps[0] = fmt.Sprintf("storage:%s", fileURL)
	}
}

func (r *SauceReportRetrier) uploadConfig(filename string) (string, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	progress.Show("Uploading runner config %s", filename)
	start := time.Now()
	resp, err := r.ProjectUploader.UploadStream(filepath.Base(filename), "", file)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().Dur("durationMs", time.Since(start)).Str("storageId", resp.ID).Msg("Runner Config uploaded.")

	return resp.ID, nil
}

func (r *SauceReportRetrier) getSauceReport(job job.Job) (saucereport.SauceReport, error) {
	content, err := r.VDCReader.GetJobAssetFileContent(context.Background(), job.ID, saucereport.SauceReportFileName, false)
	if err != nil {
		return saucereport.SauceReport{}, err
	}
	return saucereport.Parse(content)
}
