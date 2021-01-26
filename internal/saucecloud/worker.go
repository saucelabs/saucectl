package saucecloud

import "github.com/saucelabs/saucectl/internal/job"

type result struct {
	suiteName string
	browser   string
	job       job.Job
	err       error
}
