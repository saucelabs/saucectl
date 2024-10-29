package build

import "context"

// Build represents a grouping of jobs.
type Build struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Source defines the type of test device associated with the job and build.
type Source string

const (
	VDC Source = "vdc"
	RDC Source = "rdc"
)

// Service is the interface for requesting build information.
type Service interface {
	FindBuild(ctx context.Context, jobID string, realDevice bool) (Build, error)
}
