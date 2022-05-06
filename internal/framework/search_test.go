package framework

import (
	"context"
	"github.com/pkg/errors"
	"testing"
)

type MockMetadataService struct {
	MockSearch     func(context.Context, SearchOptions) (Metadata, error)
	MockFrameworks func(ctx context.Context) ([]Framework, error)
	MockVersions   func(ctx context.Context, frameworkName string) ([]Metadata, error)
}

func (m *MockMetadataService) Frameworks(ctx context.Context) ([]Framework, error) {
	if m.MockFrameworks != nil {
		return m.MockFrameworks(ctx)
	}

	return nil, nil
}

func (m *MockMetadataService) Search(ctx context.Context, opts SearchOptions) (Metadata, error) {
	if m.MockSearch != nil {
		return m.MockSearch(ctx, opts)
	}

	return Metadata{}, nil
}

func (m *MockMetadataService) Versions(ctx context.Context, frameworkName string) ([]Metadata, error) {
	if m.MockVersions != nil {
		return m.MockVersions(ctx, frameworkName)
	}

	return nil, nil
}

func TestFrameworkFind_ExactStrategy(t *testing.T) {
	var tests = []struct {
		testName         string
		expectedMessage  string
		ctx              context.Context
		svc              MetadataService
		frameworkName    string
		frameworkVersion string
	}{
		{
			"withoutFrameworkVersion",
			"framework version not defined",
			nil,
			nil,
			"buster",
			"",
		},
		{
			"invalidSearchOptions",
			"version 1.0 for buster is not available",
			nil,
			&MockMetadataService{
				MockSearch: func(context.Context, SearchOptions) (Metadata, error) {
					return Metadata{}, errors.Errorf("Bad Request: unsupported version")
				},
			},
			"buster",
			"1.0",
		},
		{
			"unknownFrameworkError",
			"framework availability unknown: unexpected error present",
			nil,
			&MockMetadataService{
				MockSearch: func(context.Context, SearchOptions) (Metadata, error) {
					return Metadata{}, errors.Errorf("unexpected error present")
				},
			},
			"buster",
			"2.0",
		},
		{
			"noErrorPresent",
			"",
			nil,
			&MockMetadataService{
				MockSearch: func(context.Context, SearchOptions) (Metadata, error) {
					return Metadata{}, nil
				},
			},
			"buster-final",
			"3.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			_, err := ExactStrategy{}.Find(tc.ctx, tc.svc, tc.frameworkName, tc.frameworkVersion)

			if err != nil && err.Error() != tc.expectedMessage {
				t.Errorf("Wrong error message displays:\nExpected: %s\nActual: %s", tc.expectedMessage, err.Error())
			}
		})
	}
}
