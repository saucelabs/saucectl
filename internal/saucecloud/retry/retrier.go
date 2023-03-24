package retry

import "github.com/saucelabs/saucectl/internal/job"

// Retrier represents the retry strategy.
type Retrier interface {
	Retry(c chan<- job.StartOptions, opt job.StartOptions, previous job.Job)
}
