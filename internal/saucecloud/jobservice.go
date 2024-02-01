package saucecloud

import (
	"context"
	"fmt"
	"time"

	cjob "github.com/saucelabs/saucectl/internal/cmd/jobs/job"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/job"
)

type JobService struct {
	VDCStarter job.Starter
	RDCStarter job.Starter

	VDCReader job.Reader
	RDCReader job.Reader

	VDCWriter job.Writer

	VDCStopper job.Stopper
	RDCStopper job.Stopper

	VDCDownloader job.ArtifactDownloader
	RDCDownloader job.ArtifactDownloader
}

func (s JobService) DownloadArtifact(jobID, suiteName string, realDevice bool) []string {
	if realDevice {
		return s.RDCDownloader.DownloadArtifact(jobID, suiteName, realDevice)
	}

	return s.VDCDownloader.DownloadArtifact(jobID, suiteName, realDevice)
}

func (s JobService) StopJob(ctx context.Context, jobID string, realDevice bool) (job.Job, error) {
	if realDevice {
		return s.RDCStopper.StopJob(ctx, jobID, realDevice)
	}

	return s.VDCStopper.StopJob(ctx, jobID, realDevice)
}

func (s JobService) UploadAsset(jobID string, realDevice bool, fileName string, contentType string, content []byte) error {
	if realDevice {
		return nil
	}

	return s.VDCWriter.UploadAsset(jobID, realDevice, fileName, contentType, content)
}

func (s JobService) ReadJob(ctx context.Context, id string, realDevice bool) (job.Job, error) {
	if realDevice {
		return s.RDCReader.ReadJob(ctx, id, realDevice)
	}

	return s.VDCReader.ReadJob(ctx, id, realDevice)
}

func (s JobService) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
	if realDevice {
		return s.RDCReader.PollJob(ctx, id, interval, timeout, realDevice)
	}

	return s.VDCReader.PollJob(ctx, id, interval, timeout, realDevice)
}

func (s JobService) GetJobAssetFileNames(ctx context.Context, jobID string, realDevice bool) ([]string, error) {
	if realDevice {
		return s.RDCReader.GetJobAssetFileNames(ctx, jobID, realDevice)
	}

	return s.VDCReader.GetJobAssetFileNames(ctx, jobID, realDevice)
}

func (s JobService) GetJobAssetFileContent(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error) {
	if realDevice {
		return s.RDCReader.GetJobAssetFileContent(ctx, jobID, fileName, realDevice)
	}

	return s.VDCReader.GetJobAssetFileContent(ctx, jobID, fileName, realDevice)
}

func (s JobService) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
	if opts.RealDevice {
		return s.RDCStarter.StartJob(ctx, opts)
	}

	return s.VDCStarter.StartJob(ctx, opts)
}

type JobCommandService struct {
	Reader      insights.Service
	UserService iam.UserService
}

func (s JobCommandService) ListJobs(ctx context.Context, jobSource string, queryOption cjob.QueryOption) (cjob.List, error) {
	user, err := s.UserService.User(ctx)
	if err != nil {
		return cjob.List{}, fmt.Errorf("failed to get user: %w", err)
	}
	return s.Reader.ListJobs(ctx, user.ID, jobSource, queryOption)
}

func (s JobCommandService) ReadJob(ctx context.Context, id string) (cjob.Job, error) {
	return s.Reader.ReadJob(ctx, id)
}
