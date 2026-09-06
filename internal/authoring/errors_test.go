package authoring

import (
	"errors"
	"fmt"
	"testing"
)

func TestAPIError_ErrorAndIs(t *testing.T) {
	err := &APIError{
		HTTPStatus: 400,
		Code:       "INVALID_QUERY",
		Detail:     "Invalid query string parameters.",
		Items: []APIErrorItem{{
			Code:    "custom",
			Path:    []any{"query", "scope"},
			Message: "scope-specific id is required",
		}},
	}
	want := "ai authoring service error (HTTP 400) INVALID_QUERY: Invalid query string parameters.; query.scope: scope-specific id is required"
	if got := err.Error(); got != want {
		t.Errorf("Error() =\n%s\nwant\n%s", got, want)
	}

	if !errors.Is(err, ErrInvalidQuery) {
		t.Error("errors.Is must match on code")
	}
	if errors.Is(err, ErrTestCaseNotFound) {
		t.Error("errors.Is must not match a different code")
	}
	wrapped := fmt.Errorf("listing variables: %w", err)
	if !errors.Is(wrapped, ErrInvalidQuery) {
		t.Error("errors.Is must see through wrapping")
	}
	if errors.Is(err, &APIError{}) {
		t.Error("a sentinel with no code must never match")
	}
}
