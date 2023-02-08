package mocks

import (
	"context"

	"github.com/saucelabs/saucectl/internal/build"
)

// FakeBuildReader mocks build.Reader
type FakeBuildReader struct {
	GetBuildIDFn func(ctx context.Context, jobID string, buildSource build.Source) (string, error)
}

func (f *FakeBuildReader) GetBuildID(ctx context.Context, jobID string, buildSource build.Source) (string, error) {
	return f.GetBuildIDFn(ctx, jobID, buildSource)
}
