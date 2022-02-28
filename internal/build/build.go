package build

import "context"

// Build represents a grouping of jobs.
type Build struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// BuildSource defines the type of test device associated with the job and build.
type BuildSource string

const (
	VDC BuildSource = "vdc"
	RDC             = "rdc"
)

// Reader is the interface for requesting build information.
type Reader interface {
	// GetBuildID returns the build id for a given job id.
	GetBuildID(ctx context.Context, jobID string, buildSource BuildSource) (string, error)
}
