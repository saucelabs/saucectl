package insights

import (
	"context"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/iam"
)

type Service interface {
	GetHistory(context.Context, iam.User, config.LaunchOrder) (JobHistory, error)
	PostTestRun(ctx context.Context, runs []TestRun) error
}
