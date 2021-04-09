package job

import (
	"context"
	"time"
)

// Reader is the interface for retrieving jobs.
type Reader interface {
	// ReadJob returns the job details.
	ReadJob(ctx context.Context, id string) (Job, error)

	// PollJob polls job details at an interval, until the job has ended, whether successfully or due to an error.
	PollJob(ctx context.Context, id string, interval time.Duration) (Job, error)

	// GetJobAssetFileNames returns all assets files available.
	GetJobAssetFileNames(ctx context.Context, jobID string) ([]string, error)

	// GetJobAssetFileContent returns the job asset file content.
	GetJobAssetFileContent(ctx context.Context, jobID, fileName string) ([]byte, error)
}
