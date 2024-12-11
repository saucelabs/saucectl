package saucecloud

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/job"
)

type JobService struct {
	RDC          http.RDCService
	Resto        http.Resto
	Webdriver    http.Webdriver
	TestComposer http.TestComposer

	ArtifactDownloadConfig config.ArtifactDownload
}

func (s JobService) DownloadArtifacts(
	jobData job.Job, isLastAttempt bool,
) []string {
	if s.skipDownload(jobData, isLastAttempt) {
		return []string{}
	}

	destDir, err := config.GetSuiteArtifactFolder(
		jobData.Name, s.ArtifactDownloadConfig,
	)
	if err != nil {
		log.Error().Msgf("Unable to create artifacts folder (%v)", err)
		return []string{}
	}

	files, err := s.ArtifactNames(
		context.Background(), jobData.ID, jobData.IsRDC,
	)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return []string{}
	}

	filepaths := fpath.MatchFiles(files, s.ArtifactDownloadConfig.Match)
	var artifacts []string

	for _, f := range filepaths {
		targetFile, err := s.downloadArtifact(
			destDir, jobData.ID, f, jobData.IsRDC,
		)
		if err != nil {
			log.Err(err).Msg("Unable to download artifacts")
			return artifacts
		}
		artifacts = append(artifacts, targetFile)
	}

	return artifacts
}

func (s JobService) StopJob(ctx context.Context, jobID string, realDevice bool) (job.Job, error) {
	if realDevice {
		return s.RDC.StopJob(ctx, jobID, realDevice)
	}

	return s.Resto.StopJob(ctx, jobID, realDevice)
}

func (s JobService) UploadArtifact(ctx context.Context, jobID string, realDevice bool, fileName string, contentType string, content []byte) error {
	if realDevice {
		return nil
	}

	return s.TestComposer.UploadAsset(ctx, jobID, fileName, contentType, content)
}

func (s JobService) Job(ctx context.Context, id string, realDevice bool) (job.Job, error) {
	if realDevice {
		return s.RDC.Job(ctx, id, realDevice)
	}

	return s.Resto.Job(ctx, id, realDevice)
}

func (s JobService) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
	if realDevice {
		return s.RDC.PollJob(ctx, id, interval, timeout, realDevice)
	}

	return s.Resto.PollJob(ctx, id, interval, timeout, realDevice)
}

func (s JobService) ArtifactNames(ctx context.Context, jobID string, realDevice bool) ([]string, error) {
	if realDevice {
		return s.RDC.ArtifactNames(ctx, jobID, realDevice)
	}

	return s.Resto.ArtifactNames(ctx, jobID, realDevice)
}

func (s JobService) Artifact(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error) {
	if realDevice {
		return s.RDC.Artifact(ctx, jobID, fileName, realDevice)
	}

	return s.Resto.Artifact(ctx, jobID, fileName, realDevice)
}

func (s JobService) StartJob(ctx context.Context, opts job.StartOptions) (job.Job, error) {
	if opts.RealDevice {
		return s.RDC.StartJob(ctx, opts)
	}

	return s.Webdriver.StartJob(ctx, opts)
}

func (s JobService) downloadArtifact(
	targetDir, jobID, fileName string, realDevice bool,
) (string, error) {
	content, err := s.Artifact(
		context.Background(), jobID, fileName, realDevice,
	)
	if err != nil {
		return "", err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return targetFile, os.WriteFile(targetFile, content, 0644)
}

func (s JobService) skipDownload(jobData job.Job, isLastAttempt bool) bool {
	return jobData.ID == "" ||
		jobData.TimedOut || !job.Done(jobData.Status) ||
		!s.ArtifactDownloadConfig.When.IsNow(jobData.Passed) ||
		(!isLastAttempt && !s.ArtifactDownloadConfig.AllAttempts)
}
