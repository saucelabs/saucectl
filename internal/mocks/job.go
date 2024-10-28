package mocks

import (
	"context"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
)

// FakeJobService resto mock
type FakeJobService struct {
	StartJobFn func(ctx context.Context, opts job.StartOptions) (string, error)
	StopJobFn  func(
		ctx context.Context, jobID string, realDevice bool,
	) (job.Job, error)
	ReadJobFn func(ctx context.Context, id string) (
		job.Job, error,
	)
	PollJobFn func(
		ctx context.Context, id string, interval time.Duration,
		timeout time.Duration,
	) (job.Job, error)

	UploadAssetFn func(
		jobID string, realDevice bool, fileName string, contentType string,
		content []byte,
	) error
	DownloadArtifactFn     func(job job.Job, isLastAttempt bool) []string
	GetJobAssetFileNamesFn func(ctx context.Context, jobID string) (
		[]string, error,
	)
	GetJobAssetFileContentFn func(
		ctx context.Context, jobID, fileName string,
	) ([]byte, error)
}

func (s *FakeJobService) StartJob(
	ctx context.Context, opts job.StartOptions,
) (jobID string, err error) {
	return s.StartJobFn(ctx, opts)
}

func (s *FakeJobService) UploadArtifact(
	jobID string, realDevice bool, fileName string, contentType string,
	content []byte,
) error {
	return s.UploadAssetFn(jobID, realDevice, fileName, contentType, content)
}

func (s *FakeJobService) StopJob(
	ctx context.Context, jobID string, realDevice bool,
) (job.Job, error) {
	return s.StopJobFn(ctx, jobID, realDevice)
}

func (s *FakeJobService) DownloadArtifacts(
	job job.Job, isLastAttempt bool,
) []string {
	return s.DownloadArtifactFn(job, isLastAttempt)
}

// ReadJob mock function
func (s *FakeJobService) Job(
	ctx context.Context, id string, _ bool,
) (job.Job, error) {
	return s.ReadJobFn(ctx, id)
}

// PollJob mock function
func (s *FakeJobService) PollJob(
	ctx context.Context, id string, interval, timeout time.Duration, _ bool,
) (job.Job, error) {
	return s.PollJobFn(ctx, id, interval, timeout)
}

// GetJobAssetFileContent mock function
func (s *FakeJobService) Artifact(
	ctx context.Context, jobID, fileName string, _ bool,
) ([]byte, error) {
	return s.GetJobAssetFileContentFn(ctx, jobID, fileName)
}

// GetJobAssetFileNames mock function
func (s *FakeJobService) ArtifactNames(
	ctx context.Context, jobID string, _ bool,
) ([]string, error) {
	return s.GetJobAssetFileNamesFn(ctx, jobID)
}
