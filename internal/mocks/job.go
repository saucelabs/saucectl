package mocks

import (
	"context"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
)

// FakeJobStarter resto mock
type FakeJobStarter struct {
	StartJobFn func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error)
}

// StartJob mock function
func (fjs *FakeJobStarter) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
	return fjs.StartJobFn(ctx, opts)
}

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

// FakeJobStopper resto mock
type FakeJobStopper struct {
	StopJobFn func(ctx context.Context, jobID string) (job.Job, error)
}

// StopJob mock function
func (fjs *FakeJobStopper) StopJob(ctx context.Context, jobID string, _ bool) (job.Job, error) {
	return fjs.StopJobFn(ctx, jobID)
}

// FakeJobWriter resto mock
type FakeJobWriter struct {
	UploadAssetFn func(jobID string, fileName string, contentType string, content []byte) error
}

// UploadAsset mock function
func (fjw *FakeJobWriter) UploadAsset(jobID string, _ bool, fileName string, contentType string, content []byte) error {
	return fjw.UploadAssetFn(jobID, fileName, contentType, content)
}
