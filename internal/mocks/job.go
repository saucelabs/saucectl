package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/job"
	"time"
)

// FakeJobStarter resto mock
type FakeJobStarter struct {
	StartJobFn                   func(ctx context.Context, opts job.StartOptions) (jobID string, err error)
	CheckFrameworkAvailabilityFn func(ctx context.Context, frameworkName string) error
}

func (fjs *FakeJobStarter) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, err error) {
	return fjs.StartJobFn(ctx, opts)
}

func (fjs *FakeJobStarter) CheckFrameworkAvailability(ctx context.Context, frameworkName string) error {
	return fjs.CheckFrameworkAvailabilityFn(ctx, frameworkName)
}

// FakeJobReader resto mock
type FakeJobReader struct {
	ReadJobFn func(ctx context.Context, id string) (job.Job, error)
	PollJobFn func(ctx context.Context, id string, interval time.Duration) (job.Job, error)
	GetJobAssetFileContentFn func (ctx context.Context, jobID, fileName string) ([]byte, error)
}

func (fjr *FakeJobReader) ReadJob(ctx context.Context, id string) (job.Job, error) {
	return fjr.ReadJobFn(ctx, id)
}

func (fjr *FakeJobReader) PollJob(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
	return fjr.PollJobFn(ctx, id, interval)
}

func (fjr *FakeJobReader) GetJobAssetFileContent(ctx context.Context, jobID, fileName string) ([]byte, error) {
	return fjr.GetJobAssetFileContentFn(ctx, jobID, fileName)
}