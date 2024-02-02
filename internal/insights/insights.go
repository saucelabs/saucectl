package insights

import (
	"context"

	ij "github.com/saucelabs/saucectl/internal/cmd/jobs/job"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/job"
)

type Service interface {
	GetHistory(context.Context, iam.User, config.LaunchOrder) (JobHistory, error)
	PostTestRun(ctx context.Context, runs []TestRun) error
	ListJobs(ctx context.Context, userID, jobType string, queryOption ij.QueryOption) ([]job.Job, error)
	ReadJob(ctx context.Context, jobID string) (job.Job, error)
}
