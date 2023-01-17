package job

import "context"

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

// Commander is the reader for saucectl jobs command interface
type Commander interface {
	// ReadJob returns the job details.
	ReadJob(ctx context.Context, id string) (Job, error)
	// ListJobs returns job list
	ListJobs(ctx context.Context, jobSource string, queryOption QueryOption) (List, error)
}
