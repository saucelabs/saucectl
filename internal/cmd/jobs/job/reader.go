package job

import (
	"context"

	"github.com/saucelabs/saucectl/internal/job"
)

// QueryOption represents the query option for listing jobs
type QueryOption struct {
	Page   int    `json:"page"`
	Size   int    `json:"size"`
	Status string `json:"status"`
}

// Reader is the interface for saucectl jobs command to fetch jobs
type Reader interface {
	// ReadJob returns the job details.
	ReadJob(ctx context.Context, id string) (job.Job, error)
	// ListJobs returns job list
	ListJobs(ctx context.Context, jobSource string, queryOption QueryOption) ([]job.Job, error)
}
