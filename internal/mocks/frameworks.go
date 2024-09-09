package mocks

import (
	"context"

	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/runtime"
)

// FakeFrameworkInfoReader is a mock for the interface framework.MetadataService.
type FakeFrameworkInfoReader struct {
	FrameworksFn func(ctx context.Context) ([]string, error)
	VersionsFn   func(ctx context.Context, frameworkName string) ([]framework.Metadata, error)
	RuntimeFn    func(ctx context.Context) ([]runtime.Runtime, error)
}

// Frameworks is a wrapper around FrameworksFn.
func (fir *FakeFrameworkInfoReader) Frameworks(ctx context.Context) ([]string, error) {
	return fir.FrameworksFn(ctx)
}

// Versions is a wrapper around VersionsFn.
func (fir *FakeFrameworkInfoReader) Versions(ctx context.Context, frameworkName string) ([]framework.Metadata, error) {
	return fir.VersionsFn(ctx, frameworkName)
}

// Runtimes is a wrapper around RuntimesFn.
func (fir *FakeFrameworkInfoReader) Runtimes(ctx context.Context) ([]runtime.Runtime, error) {
	return fir.RuntimeFn(ctx)
}
