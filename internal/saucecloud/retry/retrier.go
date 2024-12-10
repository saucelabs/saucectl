package retry

import (
	"context"

	"github.com/saucelabs/saucectl/internal/job"
)

// Retrier represents the retry strategy.
type Retrier interface {
	Retry(ctx context.Context, c chan<- job.StartOptions, opt job.StartOptions, previous job.Job)
}
