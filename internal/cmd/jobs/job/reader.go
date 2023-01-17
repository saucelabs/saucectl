package job

import (
	"context"
)

// Job represents job info for saucectl jobs cmd
type Job struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	Platform    string `json:"platform,omitempty"`
	Framework   string `json:"framework,omitempty"`
	Device      string `json:"device,omitempty"`
	BrowserName string `json:"browserName,omitempty"`
	Source      string `json:"source,omitempty"`
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

// Reader is the interface for saucectl jobs command to fetch jobs
type Reader interface {
	// ReadJob returns the job details.
	ReadJob(ctx context.Context, id string) (Job, error)
	// ListJobs returns job list
	ListJobs(ctx context.Context, jobSource string, queryOption QueryOption) (List, error)
}
