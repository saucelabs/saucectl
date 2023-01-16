package job

import (
	"context"
	"time"
)

// Reader is the interface for retrieving jobs.
type Reader interface {
	// ReadJob returns the job details.
	ReadJob(ctx context.Context, id string, realDevice bool) (Job, error)

	// PollJob polls job details at an interval, until timeout has been reached or until the job has ended, whether successfully or due to an error.
	PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (Job, error)

	// GetJobAssetFileNames returns all assets files available.
	GetJobAssetFileNames(ctx context.Context, jobID string, realDevice bool) ([]string, error)

	// GetJobAssetFileContent returns the job asset file content.
	GetJobAssetFileContent(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error)
}

// List represents the job list structure
type List struct {
	Jobs  []Job `json:"jobs"`
	Total int   `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// QueryOption represents the query option for listing jobs
type QueryOption struct {
	Page   int    `json:"page"`
	Size   int    `json:"size"`
	Status string `json:"status"`
}

// CommandReader is the reader for saucectl jobs command interface
type CommandReader interface {
	// ReadJob returns the job details.
	ReadJob(ctx context.Context, id string) (Job, error)
	// ListJobs returns job list
	ListJobs(ctx context.Context, jobSource string, queryOption QueryOption) (List, error)
}
