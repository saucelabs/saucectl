package mocks

import (
	"context"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
)

// FakeJobReader resto mock
type FakeJobReader struct {
	ReadJobFn                func(ctx context.Context, id string) (job.Job, error)
	PollJobFn                func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error)
	GetJobAssetFileNamesFn   func(ctx context.Context, jobID string) ([]string, error)
	GetJobAssetFileContentFn func(ctx context.Context, jobID, fileName string) ([]byte, error)
}

// ReadJob mock function
func (fjr *FakeJobReader) ReadJob(ctx context.Context, id string, _ bool) (job.Job, error) {
	return fjr.ReadJobFn(ctx, id)
}

// PollJob mock function
func (fjr *FakeJobReader) PollJob(ctx context.Context, id string, interval, timeout time.Duration, _ bool) (job.Job, error) {
	return fjr.PollJobFn(ctx, id, interval, timeout)
}

// GetJobAssetFileContent mock function
func (fjr *FakeJobReader) GetJobAssetFileContent(ctx context.Context, jobID, fileName string, _ bool) ([]byte, error) {
	return fjr.GetJobAssetFileContentFn(ctx, jobID, fileName)
}

// GetJobAssetFileNames mock function
func (fjr *FakeJobReader) GetJobAssetFileNames(ctx context.Context, jobID string, _ bool) ([]string, error) {
	return fjr.GetJobAssetFileNamesFn(ctx, jobID)
}
