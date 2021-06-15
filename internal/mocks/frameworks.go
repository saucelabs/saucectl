package mocks

import (
	"context"
	"errors"

	"github.com/saucelabs/saucectl/internal/framework"
)

type FakeFrameworkInfoReader struct {
	FrameworkResponse []framework.Framework
	FrameworkError    error
	VersionsResponse  []framework.Metadata
	VersionsError     error
}

func (fir *FakeFrameworkInfoReader) Search(ctx context.Context, opts framework.SearchOptions) (framework.Metadata, error) {
	return framework.Metadata{}, errors.New("mock func not implemented")
}

func (fir *FakeFrameworkInfoReader) Frameworks(ctx context.Context) ([]framework.Framework, error) {
	return fir.FrameworkResponse, fir.FrameworkError
}

func (fir *FakeFrameworkInfoReader) Versions(ctx context.Context, frameworkName string) ([]framework.Metadata, error) {
	return fir.VersionsResponse, fir.VersionsError
}