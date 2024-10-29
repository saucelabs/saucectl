package build

import (
	"context"
)

// Build represents a grouping of jobs.
type Build struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	URL string `json:"-"`
}

// Service is the interface for requesting build information.
type Service interface {
	// FindBuild returns a Build that's associated with jobID.
	FindBuild(ctx context.Context, jobID string, realDevice bool) (Build, error)
}
