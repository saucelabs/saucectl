// Package authoring provides types and a service interface for interacting
// with the Sauce Labs AI Test Authoring API (test cases generated from a
// natural language spec, grouped into test suites, and executed on demand).
package authoring

import (
	"context"
	"errors"
	"time"
)

// ErrTestCaseNotFound is returned when the requested test case does not exist.
var ErrTestCaseNotFound = errors.New("test case not found")

// ErrTestSuiteNotFound is returned when the requested test suite does not exist.
var ErrTestSuiteNotFound = errors.New("test suite not found")

// GenerateTaskStatus represents the state of an async test case generation task.
type GenerateTaskStatus string

const (
	// TaskPending indicates the generation task is still running.
	TaskPending GenerateTaskStatus = "PENDING"
	// TaskCompleted indicates the generation task finished successfully.
	TaskCompleted GenerateTaskStatus = "COMPLETED"
	// TaskFailed indicates the generation task failed.
	TaskFailed GenerateTaskStatus = "FAILED"
)

// PromptSettings describes the natural language intent used to generate a
// test case.
type PromptSettings struct {
	// Intent is the natural language description of what the test case
	// should do (1-20000 characters).
	Intent string `json:"intent"`
	// MaxSteps caps the number of steps the AI may generate (1-200).
	MaxSteps int `json:"maxSteps,omitempty"`
}

// RunSettings describes how a generated test case should be executed.
type RunSettings struct {
	TestURL      string                 `json:"testUrl,omitempty"`
	SCTunnelName string                 `json:"scTunnelName,omitempty"`
	Target       map[string]interface{} `json:"target,omitempty"`
}

// GenerateRequest is the payload sent to author a brand new test case from a
// natural language spec. Note that the Test Authoring API has no concept of
// updating an existing test case in place: every call to Generate creates a
// new TestCaseID.
type GenerateRequest struct {
	Name           string         `json:"name"`
	TestSuiteID    string         `json:"testSuiteId,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	RunSettings    RunSettings    `json:"runSettings"`
	PromptSettings PromptSettings `json:"promptSettings"`
	TimeoutMS      int            `json:"timeout,omitempty"`
}

// GenerateTask represents the status of an in-flight (or completed) test
// case generation request.
type GenerateTask struct {
	TaskID      string
	SauceJobID  string
	Status      GenerateTaskStatus
	TestCaseID  string
	ErrorCode   string
	ErrorDetail string
}

// TestCase is a single AI-authored test case.
type TestCase struct {
	ID           string
	Name         string
	TestSuiteID  string
	Tags         []string
	CreationDate time.Time
	LastUpdate   time.Time
}

// TestSuite groups a set of test cases together so they can be run as one
// unit (e.g. "regression", "smoke").
type TestSuite struct {
	ID            string
	Name          string
	Tags          []string
	TestCaseCount int
	CreationDate  time.Time
	LastUpdate    time.Time
}

// ListTestSuiteOptions filters the test suite listing.
type ListTestSuiteOptions struct {
	Search string
	Tags   []string
}

// UpdateTestSuiteOptions describes a partial update to a test suite's
// metadata and/or membership. Leave a field at its zero value to leave it
// unchanged.
type UpdateTestSuiteOptions struct {
	Name            string
	AddTestCases    []string
	RemoveTestCases []string
}

// SuiteRun is the result of triggering a test suite run. Note the Test
// Authoring API does not return the individual test case or job IDs that
// were queued -- only a count. BuildName is the only reliable correlation
// key for finding the resulting jobs in Sauce Labs afterwards.
type SuiteRun struct {
	ID        string
	OrgID     string
	TeamID    string
	UserID    string
	RunCount  int
	BuildName string
}

// ListTestCaseOptions filters/paginates a test case listing.
type ListTestCaseOptions struct {
	// TestSuiteID, if set, restricts the listing to test cases that belong
	// to this suite.
	TestSuiteID string
	Search      string
	Tags        []string
	Skip        int
	Limit       int
}

// RunTestCaseOptions describes how to execute a single test case.
type RunTestCaseOptions struct {
	BuildName    string
	SCTunnelName string
	// Targets, if set, overrides the target(s) (e.g. browser/device
	// capabilities) the test case runs against for this invocation. If nil,
	// the target(s) configured at authoring time are used. A single test
	// case can potentially run against more than one target in one call,
	// producing more than one job -- confirm this against a live account
	// before relying on it.
	Targets []map[string]interface{}
}

// TestCaseJob is a single job queued by RunTestCase. Unlike SuiteRun, this
// gives us real, immediately pollable job identifiers with no need to
// discover them after the fact via the Builds API.
type TestCaseJob struct {
	ID         string
	SauceJobID string
	Target     string
	Name       string
	URL        string
	// IsRDC indicates whether this job runs on a real device (true) or a
	// virtual device/browser (false). Use this to pick the right value for
	// job.Service's realDevice parameter when polling -- there is no need
	// for a separate global vdc/rdc flag for that purpose.
	IsRDC   bool
	Success bool
	Error   string
}

// TestCaseRun is the result of triggering a single test case run.
type TestCaseRun struct {
	ID         string
	TestCaseID string
	BuildName  string
	Jobs       []TestCaseJob
}

// Service is the interface for interacting with the Sauce Labs AI Test
// Authoring API.
type Service interface {
	// GenerateTestCase kicks off an async request to author a brand new test
	// case from a natural language spec. It always creates a new test case;
	// there is no way to target an existing one for re-authoring.
	GenerateTestCase(ctx context.Context, req GenerateRequest) (GenerateTask, error)

	// GetGenerateTask polls the status of a previously submitted generate
	// request.
	GetGenerateTask(ctx context.Context, taskID string) (GenerateTask, error)

	// GetTestCase fetches a single test case by ID.
	GetTestCase(ctx context.Context, id string) (TestCase, error)

	// ListTestCases lists test cases, optionally filtered (e.g. by
	// TestSuiteID). Returns a page of results plus the total count so
	// callers can paginate until exhausted.
	ListTestCases(ctx context.Context, opts ListTestCaseOptions) ([]TestCase, int, error)

	// DeleteTestCase deletes a test case by ID.
	DeleteTestCase(ctx context.Context, id string) error

	// RunTestCase triggers a run of a single test case, returning the job(s)
	// it queued. Prefer this over RunTestSuite when you need individual,
	// pollable job identifiers (e.g. for `saucectl run` reporting).
	RunTestCase(ctx context.Context, id string, opts RunTestCaseOptions) (TestCaseRun, error)

	// ListTestSuites lists test suites, optionally filtered.
	ListTestSuites(ctx context.Context, opts ListTestSuiteOptions) ([]TestSuite, error)

	// GetTestSuite fetches a single test suite by ID.
	GetTestSuite(ctx context.Context, id string) (TestSuite, error)

	// CreateTestSuite creates a new, empty (or pre-populated) test suite.
	CreateTestSuite(ctx context.Context, name string, tags []string) (TestSuite, error)

	// UpdateTestSuite updates a test suite's metadata and/or membership.
	UpdateTestSuite(ctx context.Context, id string, opts UpdateTestSuiteOptions) (TestSuite, error)

	// DeleteTestSuite deletes a test suite by ID.
	DeleteTestSuite(ctx context.Context, id string) error

	// RunTestSuite triggers a run of every test case currently in the suite.
	RunTestSuite(ctx context.Context, id string, buildName string) (SuiteRun, error)
}
