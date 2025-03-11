package build

import (
	"context"
)

// Build represents a grouping of jobs.
type Build struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Passed bool   `json:"passed"`
	URL    string `json:"-"`
}

type BuildResponse struct {
	Builds []Build `json:"builds"`
}

// Service is the interface for requesting build information.
type Service interface {
	// FindBuild returns a Build that's associated with jobID.
	FindBuild(ctx context.Context, jobID string, realDevice bool) (Build, error)
	ListBuilds(ctx context.Context, opts ListBuildsOptions) ([]Build, error)
}

type Source string

const (
	SourceVDC Source = "vdc" // Virtual Device Cloud
	SourceRDC Source = "rdc" // Real Device Cloud
)

type Status string

const (
	StateRunning  Status = "running"
	StateError    Status = "error"
	StateFailed   Status = "failed"
	StateComplete Status = "complete"
	StateSuccess  Status = "success"
)

var AllStates = []Status{StateRunning, StateError, StateFailed, StateComplete, StateSuccess}

type ListBuildsOptions struct {
	UserID string
	Page   int
	Size   int
	Status Status
	Source Source
}
