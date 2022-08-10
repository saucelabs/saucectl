package insights

import (
	"context"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/iam"
)

// JobHistory represents job history data structure
type JobHistory struct {
	TestCases []TestCase `json:"test_cases"`
}

// TestCase represents test case data structure
type TestCase struct {
	Name     string  `json:"name"`
	FailRate float64 `json:"fail_rate"`
}

type Service interface {
	GetHistory(context.Context, iam.User, config.LaunchOrder) (JobHistory, error)
}
