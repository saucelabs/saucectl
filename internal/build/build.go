package build

import (
	"context"
)

// Build represents a grouping of jobs.
type Build struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	URL    string `json:"-"`
}

type BuildsListResponse struct {
	Builds []Build `json:"builds"`
}

// Service is the interface for requesting build information.
type Service interface {
	// GetBuild returns a Build by build or job ID.
	GetBuild(ctx context.Context, opts GetBuildOptions) (Build, error)
	ListBuilds(ctx context.Context, opts ListBuildsOptions) ([]Build, error)
}

type Source string

const (
	SourceVDC Source = "vdc" // Virtual Device Cloud
	SourceRDC Source = "rdc" // Real Device Cloud
)

type Status string

const (
	StatusRunning  Status = "running"
	StatusError    Status = "error"
	StatusFailed   Status = "failed"
	StatusComplete Status = "complete"
	StatusSuccess  Status = "success"
)

var AllStatuses = []Status{StatusRunning, StatusError, StatusFailed, StatusComplete, StatusSuccess}

type ListBuildsOptions struct {
	UserID string
	Page   int
	Size   int
	Status Status
	Source Source
	Name   string
}

type GetBuildOptions struct {
	// The Build ID or Job ID to filter by if ByJob=True
	ID     string
	Source Source
	// If true, will find the build by querying the endpoint assuming the passed ID is a Job ID.
	ByJob bool
}
