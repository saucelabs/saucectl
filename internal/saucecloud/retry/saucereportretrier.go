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
	JobService      job.Service
	ProjectUploader storage.AppService

	Project Project
}

func (r *SauceReportRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if opt.SmartRetry.FailedOnly {
		if ok := r.retryFailedTests(&opt, previous); !ok {
			log.Info().Msg(msg.SkippingSmartRetries)
		}
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}

func (r *SauceReportRetrier) retryFailedTests(opt *job.StartOptions, previous job.Job) bool {
	if previous.Status == job.StateError {
		log.Warn().Msg(msg.UnreliableReport)
		return false
	}

	report, err := r.getSauceReport(previous)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToFetchFile, saucereport.FileName)
		return false
	}

	if err := r.Project.FilterFailedTests(opt.Name, report); err != nil {
		log.Err(err).Msg(msg.UnableToFilterFailedTests)
		return false
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if err != nil {
		log.Err(err).Msg(msg.UnableToCreateRunnerConfig)
		return false
	}
	defer os.RemoveAll(tempDir)

	runnerFile, err := zip.ArchiveRunnerConfig(r.Project, tempDir)
	if err != nil {
		log.Err(err).Msg(msg.UnableToArchiveRunnerConfig)
		return false
	}

	storageID, err := r.uploadConfig(runnerFile)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToUploadConfig, runnerFile)
		return false
	}

	if len(opt.OtherApps) == 0 {
		opt.OtherApps = []string{fmt.Sprintf("storage:%s", storageID)}
	} else {
		// FIXME(AlexP): Code smell! The order of elements in OtherApps is
		// defined by CloudRunner. While the order itself is not important, the
		// type of app is. We should not rely on the order of elements in the
		// slice. If we need to know the type, we should use a map.
		opt.OtherApps[0] = fmt.Sprintf("storage:%s", storageID)
	}

	return true
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
	resp, err := r.ProjectUploader.UploadStream(context.TODO(), storage.FileInfo{Name: filepath.Base(filename)}, file)
	progress.Stop()
	if err != nil {
		return "", err
	}
	log.Info().
		Str("duration", time.Since(start).Round(time.Second).String()).
		Str("storageId", resp.ID).
		Msg("Runner Config uploaded.")

	return resp.ID, nil
}

func (r *SauceReportRetrier) getSauceReport(job job.Job) (saucereport.SauceReport, error) {
	content, err := r.JobService.Artifact(context.Background(), job.ID, saucereport.FileName, false)
	if err != nil {
		return saucereport.SauceReport{}, err
	}
	return saucereport.Parse(content)
}
