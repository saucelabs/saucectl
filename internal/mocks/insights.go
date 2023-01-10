package mocks

import (
	"context"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/insights"
)

type FakeInsightService struct {
	GetHistoryFn  func(context.Context, iam.User, config.LaunchOrder) (insights.JobHistory, error)
	PostTestRunFn func(context.Context, []insights.TestRun) error
}

func (f FakeInsightService) GetHistory(ctx context.Context, user iam.User, cfg config.LaunchOrder) (insights.JobHistory, error) {
	return f.GetHistoryFn(ctx, user, cfg)
}

func (f FakeInsightService) PostTestRun(ctx context.Context, runs []insights.TestRun) error {
	return f.PostTestRunFn(ctx, runs)
}
