package job

import (
	"context"
	"time"
)

// Reader is the interface for retrieving jobs.
type Reader interface {
	ReadJob(ctx context.Context, id string) (Job, error)
	PollJob(ctx context.Context, id string, interval time.Duration) (Job, error)
}
