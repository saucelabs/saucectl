package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/job"
	"time"
)

// FakeJobStarter resto mock
type FakeJobStarter struct {
	StartJobFn func(ctx context.Context, opts job.StartOptions) (jobID string, err error)
}

// StartJob mock function
func (fjs *FakeJobStarter) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, err error) {
	return fjs.StartJobFn(ctx, opts)
}

// FakeJobReader resto mock
type FakeJobReader struct {
	ReadJobFn                func(ctx context.Context, id string) (job.Job, error)
	PollJobFn                func(ctx context.Context, id string, interval time.Duration) (job.Job, error)
	GetJobAssetFileContentFn func(ctx context.Context, jobID, fileName string) ([]byte, error)
}

// ReadJob mock function
func (fjr *FakeJobReader) ReadJob(ctx context.Context, id string) (job.Job, error) {
	return fjr.ReadJobFn(ctx, id)
}

// PollJob mock function
func (fjr *FakeJobReader) PollJob(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
	return fjr.PollJobFn(ctx, id, interval)
}

// GetJobAssetFileContent mock function
func (fjr *FakeJobReader) GetJobAssetFileContent(ctx context.Context, jobID, fileName string) ([]byte, error) {
	return fjr.GetJobAssetFileContentFn(ctx, jobID, fileName)
}

// FakeJobStopper resto mock
type FakeJobStopper struct {
	StopJobFn func(ctx context.Context, jobID string) (job.Job, error)
}

// StopJob mock function
func (fjs *FakeJobStopper) StopJob(ctx context.Context, jobID string) (job.Job, error) {
	return fjs.StopJobFn(ctx, jobID)
}
