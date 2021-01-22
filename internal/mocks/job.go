package mocks

import (
	"context"
	"errors"
	"github.com/saucelabs/saucectl/internal/job"
	"time"
)

// FakeJobStarter resto mock
type FakeJobStarter struct {
	StartJobFn                        func(ctx context.Context, opts job.StartOptions) (jobID string, err error)
	CheckFrameworkAvailabilitySuccess bool
}

// StartJob mock function
func (fjs *FakeJobStarter) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, err error) {
	return fjs.StartJobFn(ctx, opts)
}

// CheckFrameworkAvailability mock function
func (fjs *FakeJobStarter) CheckFrameworkAvailability(ctx context.Context, frameworkName string) error {
	if fjs.CheckFrameworkAvailabilitySuccess {
		return nil
	}
	return errors.New("framework not available")
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
