package authoring

import (
	"errors"
	"fmt"
	"strings"
)

// VariableScope is where a variable is visible.
type VariableScope string

// The variable scopes. Org-scoped variables are visible to every team; the
// others are restricted to the owning team, suite or case.
const (
	ScopeOrg       VariableScope = "org"
	ScopeTeam      VariableScope = "team"
	ScopeTestSuite VariableScope = "testSuite"
	ScopeTestCase  VariableScope = "testCase"
)

// AllVariableScopes lists every scope, in the order the service documents.
var AllVariableScopes = []VariableScope{ScopeOrg, ScopeTeam, ScopeTestSuite, ScopeTestCase}

// ParseVariableScope resolves a user-supplied scope case-insensitively.
func ParseVariableScope(s string) (VariableScope, bool) {
	for _, sc := range AllVariableScopes {
		if strings.EqualFold(string(sc), strings.TrimSpace(s)) {
			return sc, true
		}
	}
	return VariableScope(s), false
}

// ErrScopeRequiresID is returned when a suite or case scope is used without
// its identifier, and ErrScopeForbidsID when an org or team scope is given
// one. The service enforces the same pairing on listing as on creation
// (400 INVALID_QUERY), so validating client-side yields a better message.
var (
	ErrScopeRequiresID = errors.New("scope requires its identifier")
	ErrScopeForbidsID  = errors.New("scope does not accept an identifier")
)

// ValidateScopePairing checks that exactly the identifier the scope requires
// is present. An empty scope is accepted (no filter).
func ValidateScopePairing(scope VariableScope, testSuiteID, testCaseID string) error {
	switch scope {
	case ScopeTestSuite:
		if testSuiteID == "" {
			return fmt.Errorf("%w: scope=testSuite requires a test suite id", ErrScopeRequiresID)
		}
		if testCaseID != "" {
			return fmt.Errorf("%w: scope=testSuite does not accept a test case id", ErrScopeForbidsID)
		}
	case ScopeTestCase:
		if testCaseID == "" {
			return fmt.Errorf("%w: scope=testCase requires a test case id", ErrScopeRequiresID)
		}
		if testSuiteID != "" {
			return fmt.Errorf("%w: scope=testCase does not accept a test suite id", ErrScopeForbidsID)
		}
	case ScopeOrg, ScopeTeam, "":
		if testSuiteID != "" || testCaseID != "" {
			return fmt.Errorf("%w: scope=%s does not accept a test suite or test case id", ErrScopeForbidsID, scope)
		}
	default:
		return fmt.Errorf("unknown scope %q, options: %s", scope, joinScopes())
	}
	return nil
}

// joinScopes renders AllVariableScopes for error messages.
func joinScopes() string {
	names := make([]string, len(AllVariableScopes))
	for i, s := range AllVariableScopes {
		names[i] = string(s)
	}
	return strings.Join(names, ", ")
}

// Variable is a named value made available to tests, scoped to an
// organisation, team, suite or single case.
type Variable struct {
	ID          string        `json:"id"`
	OrgID       string        `json:"orgId,omitempty"`
	TeamID      string        `json:"teamId,omitempty"`
	TestSuiteID string        `json:"testSuiteId,omitempty"`
	TestCaseID  string        `json:"testCaseId,omitempty"`
	Scope       VariableScope `json:"scope"`
	// Name matches ^[a-z0-9_]+$ and is 1–255 characters.
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsSecret    bool   `json:"isSecret"`
	// Value is never returned for a secret variable — verified against the
	// live service. Commands must nonetheless blank it before rendering when
	// IsSecret is set, so a change in service behaviour cannot leak it.
	Value                string `json:"value,omitempty"`
	CreatorUserID        string `json:"creatorUserId,omitempty"`
	CreatorUserName      string `json:"creatorUserName,omitempty"`
	LastModifierUserID   string `json:"lastModifierUserId,omitempty"`
	LastModifierUserName string `json:"lastModifierUserName,omitempty"`
	CreationDate         string `json:"creationDate,omitempty"`
	// LastUpdate is the optimistic-concurrency token: it must be echoed
	// byte-for-byte as expectedLastUpdate on update and delete, which is why
	// it is a string and never parsed into time.Time.
	LastUpdate string `json:"lastUpdate,omitempty"`
}

// ListVariablesOptions filters a variable listing. Scope pairing is validated
// client-side via ValidateScopePairing.
type ListVariablesOptions struct {
	ListOptions
	Scope       VariableScope
	TestSuiteID string
	TestCaseID  string
	Search      string
}

// CreateVariableOptions is the request to create a variable.
type CreateVariableOptions struct {
	Scope       VariableScope `json:"scope"`
	TestSuiteID string        `json:"testSuiteId,omitempty"`
	TestCaseID  string        `json:"testCaseId,omitempty"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	IsSecret    bool          `json:"isSecret"`
	Value       string        `json:"value"`
}

// UpdateVariableOptions is the request to change a variable. Pointer fields
// are tri-state: nil leaves the field unchanged. ExpectedLastUpdate is
// mandatory and goes in the body here — but in the query string on delete.
type UpdateVariableOptions struct {
	Name               *string `json:"name,omitempty"`
	Description        *string `json:"description,omitempty"`
	Value              *string `json:"value,omitempty"`
	IsSecret           *bool   `json:"isSecret,omitempty"`
	ExpectedLastUpdate string  `json:"expectedLastUpdate"`
}
