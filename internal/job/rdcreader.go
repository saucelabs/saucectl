package job

import (
	"context"
	"time"
)

// Reader is a specialized interface for retrieving jobs from RDC, extending functionality from the baseline.
type RDCReader interface {
	PollDevicesState(ctx context.Context, id string, interval time.Duration) (string, error)
	Reader
}
