package framework

import "context"

// Framework represents a test framework (e.g. cypress).
type Framework struct {
	Name    string
	Version string
}

// ImageLocator represents an interface for retrieving (docker) images for a given framework.
type ImageLocator interface {
	GetImage(ctx context.Context, f Framework) (string, error)
}
