package authoring

import (
	"errors"
	"testing"
)

func TestValidateScopePairing(t *testing.T) {
	tests := []struct {
		name    string
		scope   VariableScope
		suite   string
		tc      string
		wantErr error
	}{
		{name: "org without ids", scope: ScopeOrg},
		{name: "team without ids", scope: ScopeTeam},
		{name: "no scope, no ids", scope: ""},
		{name: "testSuite with suite id", scope: ScopeTestSuite, suite: "s"},
		{name: "testCase with case id", scope: ScopeTestCase, tc: "c"},
		{name: "testSuite missing id", scope: ScopeTestSuite, wantErr: ErrScopeRequiresID},
		{name: "testCase missing id", scope: ScopeTestCase, wantErr: ErrScopeRequiresID},
		{name: "testSuite with case id", scope: ScopeTestSuite, suite: "s", tc: "c", wantErr: ErrScopeForbidsID},
		{name: "org with suite id", scope: ScopeOrg, suite: "s", wantErr: ErrScopeForbidsID},
		{name: "no scope with case id", scope: "", tc: "c", wantErr: ErrScopeForbidsID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScopePairing(tt.scope, tt.suite, tt.tc)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
	if err := ValidateScopePairing("galaxy", "", ""); err == nil {
		t.Error("unknown scope must be rejected")
	}
}

func TestParseVariableScope(t *testing.T) {
	if s, ok := ParseVariableScope("TESTSUITE"); !ok || s != ScopeTestSuite {
		t.Errorf("got %v %v", s, ok)
	}
	if _, ok := ParseVariableScope("nope"); ok {
		t.Error("unknown scope accepted")
	}
}

func TestUpdateTestSuiteOptions_Validate(t *testing.T) {
	if err := (UpdateTestSuiteOptions{}).Validate(); !errors.Is(err, ErrEmptyUpdate) {
		t.Errorf("empty update: got %v", err)
	}
	if err := (UpdateTestSuiteOptions{TestCases: []string{"a"}, AddTestCases: []string{"b"}}).Validate(); !errors.Is(err, ErrTestCasesExclusive) {
		t.Errorf("exclusive violation: got %v", err)
	}
	if err := (UpdateTestSuiteOptions{Name: "x"}).Validate(); err != nil {
		t.Errorf("name only should be valid: %v", err)
	}
	if err := (UpdateTestSuiteOptions{AddTestCases: []string{"b"}, RemoveTestCases: []string{"c"}}).Validate(); err != nil {
		t.Errorf("add+remove should be valid: %v", err)
	}
}
