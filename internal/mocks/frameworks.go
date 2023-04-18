package mocks

import (
	"context"

	"github.com/saucelabs/saucectl/internal/framework"
)

// FakeFrameworkInfoReader is a mock for the interface framework.MetadataService.
type FakeFrameworkInfoReader struct {
	FrameworksFn func(ctx context.Context) ([]string, error)
	VersionsFn   func(ctx context.Context, frameworkName string) ([]framework.Metadata, error)
}

// Frameworks is a wrapper around FrameworksFn.
func (fir *FakeFrameworkInfoReader) Frameworks(ctx context.Context) ([]string, error) {
	return fir.FrameworksFn(ctx)
}

// Versions is a wrapper around VersionsFn.
func (fir *FakeFrameworkInfoReader) Versions(ctx context.Context, frameworkName string) ([]framework.Metadata, error) {
	return fir.VersionsFn(ctx, frameworkName)
}
