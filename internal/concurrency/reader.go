package concurrency

import (
	"context"
)

// Reader is the interface for retrieving account wide concurrency settings from Sauce Labs.
type Reader interface {
	// ReadAllowedCCY returns the allowed (max) concurrency for the current account.
	ReadAllowedCCY(ctx context.Context) (int, error)
}
