package insights

import (
	"context"

	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/job"
)

type Service interface {
	GetHistory(ctx context.Context, user iam.User, sortBy string) (JobHistory, error)
	PostTestRun(ctx context.Context, runs []TestRun) error
	ListJobs(ctx context.Context, opts ListJobsOptions) ([]job.Job, error)
	ReadJob(ctx context.Context, id string) (job.Job, error)
}

// ListJobsOptions represents the query option for listing jobs
type ListJobsOptions struct {
	UserID string
	Page   int
	Size   int
	Status string
	Source string
}
