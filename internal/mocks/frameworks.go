package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/framework"
)

// FakeFrameworkInfoReader is a mock for the interface framework.MetadataService.
type FakeFrameworkInfoReader struct {
	FrameworksFn func(ctx context.Context) ([]framework.Framework, error)
	VersionsFn  func(ctx context.Context, frameworkName string) ([]framework.Metadata, error)
	SearchFn    func(ctx context.Context, opts framework.SearchOptions) (framework.Metadata, error)
}

// Search is wrapper around SearchFn.
func (fir *FakeFrameworkInfoReader) Search(ctx context.Context, opts framework.SearchOptions) (framework.Metadata, error) {
	return fir.SearchFn(ctx, opts)
}

// Frameworks is a wrapper around FrameworksFn.
func (fir *FakeFrameworkInfoReader) Frameworks(ctx context.Context) ([]framework.Framework, error) {
	return fir.FrameworksFn(ctx)
}

// Versions is a wrapper around VersionsFn.
func (fir *FakeFrameworkInfoReader) Versions(ctx context.Context, frameworkName string) ([]framework.Metadata, error) {
	return fir.VersionsFn(ctx, frameworkName)
}
