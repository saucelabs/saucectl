// Package aiauthoring defines the domain types and service interfaces for
// the Sauce Labs AI Test Authoring backend.
package aiauthoring

import (
	"context"
	"encoding/json"
)

// TestCase describes a saved AI-authored test case.
type TestCase struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// The backend reports timestamps as createdAt/updatedAt or
	// creationDate/lastUpdateDate depending on the endpoint. Use Created()
	// and Updated() to read them uniformly.
	CreatedAt       string       `json:"createdAt,omitempty"`
	UpdatedAt       string       `json:"updatedAt,omitempty"`
	CreationDate    string       `json:"creationDate,omitempty"`
	LastUpdateDate  string       `json:"lastUpdateDate,omitempty"`
	UserID          string       `json:"userId,omitempty"`
	TeamID          string       `json:"teamId,omitempty"`
	CreatorUserName string       `json:"creatorUserName,omitempty"`
	Status          string       `json:"status,omitempty"`
	Framework       string       `json:"framework,omitempty"`
	Description     string       `json:"description,omitempty"`
	TestSuiteID     string       `json:"testSuiteId,omitempty"`
	TestSuiteName   string       `json:"testSuiteName,omitempty"`
	RunSettings     *RunSettings `json:"runSettings,omitempty"`
}

// Created returns the creation timestamp, regardless of which field the
// backend used to report it.
func (t TestCase) Created() string {
	if t.CreatedAt != "" {
		return t.CreatedAt
	}
	return t.CreationDate
}

// Updated returns the last-update timestamp, regardless of which field the
// backend used to report it.
func (t TestCase) Updated() string {
	if t.UpdatedAt != "" {
		return t.UpdatedAt
	}
	return t.LastUpdateDate
}

// RunSettings holds the defaults a test case was authored with.
type RunSettings struct {
	TestURL string `json:"testUrl,omitempty"`
	// SCTunnelName is a pointer to tell an absent tunnel (nil) apart from an
	// authored-but-empty one (""). The backend rejects runs whose effective
	// tunnel name is "" with SC_TUNNEL_NOT_FOUND, so an empty one has to be
	// explicitly cleared when starting a run.
	SCTunnelName  *string    `json:"scTunnelName,omitempty"`
	PrimaryTarget *RunTarget `json:"primaryTarget,omitempty"`
}

// RunCapabilities is an opaque, W3C-style capabilities map. The backend owns
// its schema; saucectl passes it through untouched.
type RunCapabilities map[string]any

// RunTarget is a single platform target for a test case run.
type RunTarget struct {
	Capabilities RunCapabilities `json:"capabilities"`
}

// ListOptions are the filters accepted by the test case listing endpoint.
type ListOptions struct {
	Search      string
	Skip        int
	Limit       int
	StartDate   string
	EndDate     string
	UserID      string
	TeamID      string
	TestSuiteID string
}

// TestSuite describes a saved suite of AI-authored test cases. The test
// cases themselves are listed via ListOptions.TestSuiteID.
type TestSuite struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Tags            []string `json:"tags,omitempty"`
	TestCaseCount   int      `json:"testCaseCount"`
	RunCount        int      `json:"runCount"`
	CreationDate    string   `json:"creationDate,omitempty"`
	LastUpdate      string   `json:"lastUpdate,omitempty"`
	CreatorUserName string   `json:"creatorUserName,omitempty"`
}

// List is one page of test cases.
type List struct {
	Items []TestCase `json:"items"`
	Total int        `json:"total"`
}

// RunRequest is the payload for starting a cloud run of a saved test case.
type RunRequest struct {
	BuildName string `json:"buildName,omitempty"`
	// SCTunnelName is the name of a Sauce Connect tunnel to run through.
	// When empty, the test case's authored tunnel (if any) applies.
	SCTunnelName string `json:"scTunnelName,omitempty"`
	// ClearTunnel sends an explicit null tunnel, overriding a tunnel stored
	// in the test case's authored run settings.
	ClearTunnel bool `json:"-"`
	// Targets are the platforms to run against. If empty, the backend
	// decides based on the test case's authored settings.
	Targets []RunTarget `json:"targets,omitempty"`
}

// MarshalJSON emits an explicit "scTunnelName": null when ClearTunnel is set
// and no tunnel name is given; omitempty alone cannot express that.
func (r RunRequest) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if r.BuildName != "" {
		m["buildName"] = r.BuildName
	}
	if r.SCTunnelName != "" {
		m["scTunnelName"] = r.SCTunnelName
	} else if r.ClearTunnel {
		m["scTunnelName"] = nil
	}
	if len(r.Targets) > 0 {
		m["targets"] = r.Targets
	}
	return json.Marshal(m)
}

// TestCaseRunJob is a single job spawned by a test case run. Its ID is
// internal to the AI Authoring backend; SauceJobID, URL and Success are
// filled in asynchronously while the run progresses.
type TestCaseRunJob struct {
	ID         string `json:"id"`
	SauceJobID string `json:"sauceJobId,omitempty"`
	Name       string `json:"name"`
	Target     struct {
		Capabilities RunCapabilities `json:"capabilities"`
		IsRDC        bool            `json:"isRdc"`
	} `json:"target"`
	URL     string `json:"url,omitempty"`
	Success *bool  `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Done returns true once the job reached a final state.
func (j TestCaseRunJob) Done() bool {
	return j.Success != nil || j.Error != ""
}

// Passed returns true if the job finished successfully.
func (j TestCaseRunJob) Passed() bool {
	return j.Success != nil && *j.Success
}

// TestCaseRun describes a started cloud run of a test case.
type TestCaseRun struct {
	ID           string           `json:"id"`
	TestCaseID   string           `json:"testCaseId"`
	Build        string           `json:"build,omitempty"`
	Jobs         []TestCaseRunJob `json:"jobs"`
	CreationDate string           `json:"creationDate,omitempty"`
	TestURL      string           `json:"testUrl,omitempty"`
}

// TestCaseService manages saved test cases.
type TestCaseService interface {
	ListTestCases(ctx context.Context, opts ListOptions) (List, error)
	GetTestCase(ctx context.Context, id string) (TestCase, error)
	RenameTestCase(ctx context.Context, id, name string) (TestCase, error)
	DeleteTestCase(ctx context.Context, id string) error
}

// TestSuiteReader fetches saved test suites.
type TestSuiteReader interface {
	GetTestSuite(ctx context.Context, id string) (TestSuite, error)
}

// TestCaseRunner starts and observes cloud runs of saved test cases.
type TestCaseRunner interface {
	RunTestCase(ctx context.Context, id string, req RunRequest) (TestCaseRun, error)
	GetTestCaseRun(ctx context.Context, testCaseID, runID string) (TestCaseRun, error)
}

// EntitlementReader reports whether AI Test Authoring is enabled for an
// organization. The feature is gated fail-closed on this entitlement.
type EntitlementReader interface {
	IsAIAuthoringEnabled(ctx context.Context, orgID string) (bool, error)
}
