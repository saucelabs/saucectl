package insights

import (
	"context"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/job"
)

type Service interface {
	GetHistory(context.Context, iam.User, config.LaunchOrder) (JobHistory, error)
	PostTestRun(ctx context.Context, runs []TestRun) error
	ListJobs(ctx context.Context, userID, source string, opts ListJobsOptions) ([]job.Job, error)
	ReadJob(ctx context.Context, id string) (job.Job, error)
}

// ListJobsOptions represents the query option for listing jobs
type ListJobsOptions struct {
	Page   int    `json:"page"`
	Size   int    `json:"size"`
	Status string `json:"status"`
}
