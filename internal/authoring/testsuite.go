package authoring

import "errors"

// TestSuite is a named collection of test cases that can be run, scheduled
// and reported on together. Its identifier is a dashless 32-character hex
// UUID — a different shape from a test case's 24-character ObjectId.
type TestSuite struct {
	ID                   string   `json:"id"`
	OrgID                string   `json:"orgId,omitempty"`
	TeamID               string   `json:"teamId,omitempty"`
	Name                 string   `json:"name"`
	Tags                 []string `json:"tags"`
	CreationDate         string   `json:"creationDate,omitempty"`
	LastUpdate           string   `json:"lastUpdate,omitempty"`
	CreatorUserID        string   `json:"creatorUserId,omitempty"`
	CreatorUserName      string   `json:"creatorUserName,omitempty"`
	LastModifierUserID   string   `json:"lastModifierUserId,omitempty"`
	LastModifierUserName string   `json:"lastModifierUserName,omitempty"`
	// TestCaseCount matches GET /testcases?testSuiteId= exactly (research
	// R-003), which is what makes client-side suite expansion viable.
	TestCaseCount int `json:"testCaseCount"`
}

// ListTestSuitesOptions filters a suite listing.
type ListTestSuitesOptions struct {
	ListOptions
	IDs       []string
	Search    string
	StartDate string
	EndDate   string
	UserID    string
	TeamID    string
}

// CreateTestSuiteOptions is the request to create a suite.
type CreateTestSuiteOptions struct {
	Name      string   `json:"name"`
	Tags      []string `json:"tags,omitempty"`
	TestCases []string `json:"testCases,omitempty"`
}

// UpdateTestSuiteOptions is the request to update a suite. Fields left at
// their zero value are omitted and therefore unchanged. TestCases replaces the
// membership wholesale and is mutually exclusive with the incremental
// AddTestCases / RemoveTestCases — enforced by Validate before any request is
// sent rather than left to the service.
type UpdateTestSuiteOptions struct {
	Name            string   `json:"name,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	TestCases       []string `json:"testCases,omitempty"`
	AddTestCases    []string `json:"addTestCases,omitempty"`
	RemoveTestCases []string `json:"removeTestCases,omitempty"`
}

// ErrEmptyUpdate is returned when an update carries no change at all.
var ErrEmptyUpdate = errors.New("nothing to update: specify at least one change")

// ErrTestCasesExclusive is returned when wholesale and incremental membership
// changes are combined in one update.
var ErrTestCasesExclusive = errors.New("testCases cannot be combined with addTestCases or removeTestCases")

// Validate enforces the update's invariants client-side.
func (o UpdateTestSuiteOptions) Validate() error {
	if o.Name == "" && o.Tags == nil && o.TestCases == nil && o.AddTestCases == nil && o.RemoveTestCases == nil {
		return ErrEmptyUpdate
	}
	if o.TestCases != nil && (o.AddTestCases != nil || o.RemoveTestCases != nil) {
		return ErrTestCasesExclusive
	}
	return nil
}

// SuiteRun is the response to queuing a suite run. It carries a count and the
// build name only — no per-case run identifiers — which is why the runner
// expands suites client-side instead (research R-002).
type SuiteRun struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId,omitempty"`
	TeamID    string `json:"teamId,omitempty"`
	UserID    string `json:"userId,omitempty"`
	RunCount  int    `json:"runCount"`
	BuildName string `json:"buildName"`
}
