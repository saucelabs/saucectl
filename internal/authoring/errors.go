package authoring

import (
	"fmt"
	"strings"
)

// APIError is the decoded error envelope returned by the AI Authoring
// service. Every non-2xx response carries
//
//	{"error": {"code": "...", "detail": "...", "data": [...]}}
//
// Code is the machine-readable identifier the sentinels below compare on;
// Detail is the human-readable summary. Items holds the undocumented
// error.data[] array, which is where the service puts the sentence that
// actually says what to fix (research R-007): for INVALID_QUERY the detail is
// merely "Invalid query string parameters." while data[0].message names the
// missing parameter. Dropping it would leave users with an unactionable error.
type APIError struct {
	// HTTPStatus is the response status code. Zero when the error was
	// constructed as a sentinel rather than decoded from a response.
	HTTPStatus int `json:"-"`
	// Code is the service's machine-readable error code, e.g. TEST_CASE_NOT_FOUND.
	Code string `json:"code"`
	// Detail is the service's human-readable summary.
	Detail string `json:"detail,omitempty"`
	// Items carries the per-field messages from the undocumented data[] array.
	Items []APIErrorItem `json:"data,omitempty"`
}

// APIErrorItem is one entry of the undocumented error.data[] array. Path is
// kept as raw values because the service mixes strings and integers in it
// (object keys and array indices).
type APIErrorItem struct {
	Code    string `json:"code,omitempty"`
	Path    []any  `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

// Error renders the code, the detail and every per-field message, so the one
// sentence that explains the problem is never hidden behind a generic summary.
func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("ai authoring service error")
	if e.HTTPStatus != 0 {
		fmt.Fprintf(&b, " (HTTP %d)", e.HTTPStatus)
	}
	if e.Code != "" {
		fmt.Fprintf(&b, " %s", e.Code)
	}
	if e.Detail != "" {
		fmt.Fprintf(&b, ": %s", e.Detail)
	}
	for _, it := range e.Items {
		if it.Message == "" {
			continue
		}
		b.WriteString("; ")
		if len(it.Path) > 0 {
			parts := make([]string, 0, len(it.Path))
			for _, p := range it.Path {
				parts = append(parts, fmt.Sprint(p))
			}
			fmt.Fprintf(&b, "%s: ", strings.Join(parts, "."))
		}
		b.WriteString(it.Message)
	}
	return b.String()
}

// Is reports whether target is an *APIError with the same Code, which makes
// errors.Is(err, ErrTestCaseNotFound) work regardless of HTTP status or detail
// text. A sentinel with an empty Code never matches.
func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	if !ok || t.Code == "" {
		return false
	}
	return t.Code == e.Code
}

// Sentinels for every error code the service declares. They carry only a
// Code, so they compare by code through errors.Is; the decoded error that
// reaches the caller still holds the status, detail and items. Named ErrXxx
// for the errname linter.
var (
	ErrAppNotFound                  = &APIError{Code: "APP_NOT_FOUND"}
	ErrCodeGenerationTargetInvalid  = &APIError{Code: "CODE_GENERATION_TARGET_INVALID"}
	ErrCodeGenerationTargetNotFound = &APIError{Code: "CODE_GENERATION_TARGET_NOT_FOUND"}
	ErrFileNotFound                 = &APIError{Code: "FILE_NOT_FOUND"}
	ErrInvalidBody                  = &APIError{Code: "INVALID_BODY"}
	ErrInvalidParams                = &APIError{Code: "INVALID_PARAMS"}
	ErrInvalidQuery                 = &APIError{Code: "INVALID_QUERY"}
	ErrInvalidTestCases             = &APIError{Code: "INVALID_TEST_CASES"}
	ErrInvalidTestSuites            = &APIError{Code: "INVALID_TEST_SUITES"}
	ErrNoRunTargets                 = &APIError{Code: "NO_RUN_TARGETS"}
	ErrRunningUserNotFound          = &APIError{Code: "RUNNING_USER_NOT_FOUND"}
	ErrSCTunnelNotFound             = &APIError{Code: "SC_TUNNEL_NOT_FOUND"}
	ErrTestCasesNotFound            = &APIError{Code: "TEST_CASES_NOT_FOUND"}
	ErrTestCaseEmpty                = &APIError{Code: "TEST_CASE_EMPTY"}
	ErrGenerationTaskNotFound       = &APIError{Code: "TEST_CASE_GENERATION_TASK_NOT_FOUND"}
	ErrTestCaseNotFound             = &APIError{Code: "TEST_CASE_NOT_FOUND"}
	ErrTestCaseRevisionNotFound     = &APIError{Code: "TEST_CASE_REVISION_NOT_FOUND"}
	ErrTestCaseRunNotFound          = &APIError{Code: "TEST_CASE_RUN_NOT_FOUND"}
	ErrTestScheduleNotFound         = &APIError{Code: "TEST_SCHEDULE_NOT_FOUND"}
	ErrTestSuitesNotFound           = &APIError{Code: "TEST_SUITES_NOT_FOUND"}
	ErrTestSuiteNotFound            = &APIError{Code: "TEST_SUITE_NOT_FOUND"}
	ErrTestSuiteNoRunJobs           = &APIError{Code: "TEST_SUITE_NO_RUN_JOBS"}
	ErrUnauthorized                 = &APIError{Code: "UNAUTHORIZED"}
	ErrUnknownBackendType           = &APIError{Code: "UNKNOWN_BACKEND_TYPE"}
	ErrVariableForbidden            = &APIError{Code: "VARIABLE_FORBIDDEN"}
	ErrVariableNameConflict         = &APIError{Code: "VARIABLE_NAME_CONFLICT"}
	ErrVariableNotFound             = &APIError{Code: "VARIABLE_NOT_FOUND"}
	ErrVariableScopeInvalid         = &APIError{Code: "VARIABLE_SCOPE_INVALID"}
	// ErrVariableVersionConflict is the 412 returned when expectedLastUpdate no
	// longer matches: somebody changed the variable since it was read.
	ErrVariableVersionConflict = &APIError{Code: "VARIABLE_VERSION_CONFLICT"}
)
