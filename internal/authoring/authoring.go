// Package authoring holds the domain model and service interfaces for the
// Sauce Labs AI Test Authoring service, reached over the AI Authoring API
// (/ai-authoring/v1). The HTTP implementation lives in internal/http; the
// commands live in internal/cmd/authoring; the `kind: authoring` runner is in
// this package alongside its configuration.
//
// Everything about the remote service that this package encodes was verified
// against the live API rather than taken from its published specification —
// several observed behaviours contradict that specification and are recorded
// where the affected code lives (see specs/001-ai-test-authoring/research.md).
package authoring

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// DefaultPageSize is the page size used when walking a listing exhaustively.
// Listings are heavy (a test case is ~13 KB because every revision and step is
// inlined), so this stays well below the service's uncapped limit.
const DefaultPageSize = 100

// maxListPages bounds ListAll so a service that ignored `skip` and kept
// returning full pages could not loop forever. At DefaultPageSize this allows
// 100,000 items, far beyond any organisation observed.
const maxListPages = 1000

// ErrListTooLong is returned by ListAll when maxListPages is exhausted before
// the service signals the end of the listing.
var ErrListTooLong = errors.New("listing did not end within the maximum number of pages")

// List is one page of a listing endpoint. Every listing in the service returns
// the same envelope: the items on this page plus the total across all pages.
type List[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// ListOptions is the pagination shared by every listing endpoint.
type ListOptions struct {
	// Skip is the offset of the first item to return.
	Skip int
	// Limit is the page size. It is a pointer because the service treats
	// limit=0 as a meaningful count-only request ({"total": N, "items": []}),
	// so "not set" and "zero" must be distinguishable: nil is not sent,
	// zero is.
	Limit *int
}

// NewListOptions returns options for one page of the given size at the given
// offset. A convenience for callers that always send a limit.
func NewListOptions(skip, limit int) ListOptions {
	return ListOptions{Skip: skip, Limit: &limit}
}

// ListAll walks every page of a listing by calling fetch with successive
// offsets until the returned page is short, the reported total is reached, or
// maxListPages is exhausted. pageSize <= 0 uses DefaultPageSize.
func ListAll[T any](ctx context.Context, pageSize int, fetch func(ctx context.Context, opts ListOptions) (List[T], error)) ([]T, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	var all []T
	for page := 0; page < maxListPages; page++ {
		l, err := fetch(ctx, NewListOptions(page*pageSize, pageSize))
		if err != nil {
			return all, err
		}
		all = append(all, l.Items...)

		if len(l.Items) < pageSize || len(all) >= l.Total {
			return all, nil
		}
	}

	return all, fmt.Errorf("%w (%d pages of %d)", ErrListTooLong, maxListPages, pageSize)
}

// TestCaseService covers the test case endpoints, including runs, generation
// and code export.
type TestCaseService interface {
	// ListTestCases returns one page of test cases matching opts.
	ListTestCases(ctx context.Context, opts ListTestCasesOptions) (List[TestCase], error)
	// GetTestCase returns a single test case with every revision and step.
	GetTestCase(ctx context.Context, id string) (TestCase, error)
	// DeleteTestCase removes a test case. It does not fail if runs exist.
	DeleteTestCase(ctx context.Context, id string) error
	// RenameTestCase changes a test case's name and returns the updated case.
	RenameTestCase(ctx context.Context, id, name string) (TestCase, error)
	// RunTestCase starts a run of the test case's latest revision, or of the
	// given revision when revisionID is non-empty. The returned Run is not
	// terminal: poll it with GetRun using the run's own TestCaseID.
	RunTestCase(ctx context.Context, id, revisionID string, opts RunOptions) (Run, error)
	// ListRuns returns one page of the runs that belong to testCaseID. The
	// implementation must send testCaseID as a query parameter — the path
	// parameter alone is ignored by the service and returns every run in the
	// organisation (research R-004).
	ListRuns(ctx context.Context, testCaseID string, opts ListRunsOptions) (List[Run], error)
	// GetRun returns a single run. testCaseID must be the run's own
	// TestCaseID; the service enforces it here even though it ignores it on
	// the list endpoint.
	GetRun(ctx context.Context, testCaseID, runID string) (Run, error)
	// ListTags returns every distinct tag in the organisation. Tags are
	// case-sensitive: "Login" and "login" are different tags.
	ListTags(ctx context.Context) ([]string, error)
	// Generate starts an asynchronous authoring task from a plain-language
	// intent and returns its identifiers.
	Generate(ctx context.Context, opts GenerateOptions) (GenerateTask, error)
	// GenerationStatus returns the current state of an authoring task,
	// including the steps captured so far.
	GenerationStatus(ctx context.Context, taskID string) (GenerationState, error)
	// Code exports the latest revision as source for the given target. Use
	// CodeTargets to discover valid targets first.
	Code(ctx context.Context, id, target string) (string, error)
	// CodeTargets returns the export targets available for the test case.
	CodeTargets(ctx context.Context, id string) ([]string, error)
}

// TestSuiteService covers the test suite endpoints.
type TestSuiteService interface {
	ListTestSuites(ctx context.Context, opts ListTestSuitesOptions) (List[TestSuite], error)
	GetTestSuite(ctx context.Context, id string) (TestSuite, error)
	CreateTestSuite(ctx context.Context, opts CreateTestSuiteOptions) (TestSuite, error)
	UpdateTestSuite(ctx context.Context, id string, opts UpdateTestSuiteOptions) (TestSuite, error)
	// DeleteTestSuite removes a suite. When deleteTestCases is true the
	// service also deletes every test case in the suite.
	DeleteTestSuite(ctx context.Context, id string, deleteTestCases bool) error
	// RunTestSuite queues a run for every case in the suite. The response
	// carries only a count and the build name — there is nothing to poll, so
	// this is fire-and-forget (research R-002).
	RunTestSuite(ctx context.Context, id, buildName string) (SuiteRun, error)
}

// ScheduleService covers the test schedule endpoints.
type ScheduleService interface {
	ListSchedules(ctx context.Context, opts ListSchedulesOptions) (List[TestSchedule], error)
	GetSchedule(ctx context.Context, id string) (TestSchedule, error)
	CreateSchedule(ctx context.Context, opts CreateScheduleOptions) (TestSchedule, error)
	UpdateSchedule(ctx context.Context, id string, opts UpdateScheduleOptions) (TestSchedule, error)
	DeleteSchedule(ctx context.Context, id string) error
}

// VariableService covers the variable endpoints.
type VariableService interface {
	ListVariables(ctx context.Context, opts ListVariablesOptions) (List[Variable], error)
	GetVariable(ctx context.Context, id string) (Variable, error)
	CreateVariable(ctx context.Context, opts CreateVariableOptions) (Variable, error)
	// UpdateVariable changes a variable. opts.ExpectedLastUpdate must echo the
	// LastUpdate of a prior read; a mismatch is ErrVariableVersionConflict.
	UpdateVariable(ctx context.Context, id string, opts UpdateVariableOptions) (Variable, error)
	// DeleteVariable removes a variable. expectedLastUpdate must echo the
	// LastUpdate of a prior read; a mismatch is ErrVariableVersionConflict.
	DeleteVariable(ctx context.Context, id, expectedLastUpdate string) error
}

// ArtifactService downloads files captured during authoring, such as step
// screenshots.
type ArtifactService interface {
	// DownloadArtifact streams the artifact with the given identifier. The
	// identifier is the last path segment of a step's screenshot URL, not the
	// URL itself. The response carries no content type, so the caller decides
	// the file name. The caller must close the returned reader.
	DownloadArtifact(ctx context.Context, id string) (io.ReadCloser, error)
}

// EntitlementReader answers whether an organisation may use AI authoring.
// This is served by a platform API outside the authoring service (research
// R-009); without it every command would fail with an opaque 401/403.
type EntitlementReader interface {
	IsAIAuthoringEnabled(ctx context.Context, orgID string) (bool, error)
}
