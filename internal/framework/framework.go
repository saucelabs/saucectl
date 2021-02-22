package framework

import "context"

// Framework represents a test framework (e.g. cypress).
type Framework struct {
	Name    string
	Version string
}

// MetadataService represents an interface for retrieving framework metadata.
type MetadataService interface {
	Search(ctx context.Context, opts SearchOptions) (Metadata, error)
}

// SearchOptions represents read query options for MetadataService.Search().
type SearchOptions struct {
	Name             string
	FrameworkVersion string
}

// Metadata represents test runner metadata.
type Metadata struct {
	FrameworkName      string
	FrameworkVersion   string
	CloudRunnerVersion string
	DockerImage        string
	GitRelease         string
}
