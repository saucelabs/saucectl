package insights

import (
	"context"
	"time"

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

// TestRun represents a
type TestRun struct {
	ID           string     `json:"id,omitempty"`
	Name         string     `json:"name,omitempty"`
	UserID       string     `json:"user_id,omitempty"`
	OrgID        string     `json:"org_id,omitempty"`
	TeamID       string     `json:"team_id,omitempty"`
	GroupID      string     `json:"group_id,omitempty"`
	AuthorID     string     `json:"author_id,omitempty"`
	PathName     string     `json:"path_name,omitempty"`
	BuildID      string     `json:"build_id,omitempty"`
	BuildName    string     `json:"build_name,omitempty"`
	CreationTime time.Time  `json:"creation_time,omitempty"`
	StartTime    time.Time  `json:"start_time,omitempty"`
	EndTime      time.Time  `json:"end_time,omitempty"`
	Duration     int        `json:"duration,omitempty"`
	Browser      string     `json:"browser,omitempty"`
	Device       string     `json:"device,omitempty"`
	OS           string     `json:"os,omitempty"`
	AppName      string     `json:"app_name,omitempty"`
	Status       string     `json:"status,omitempty"`
	Platform     string     `json:"platform,omitempty"`
	Type         string     `json:"type,omitempty"`
	Framework    string     `json:"framework,omitempty"`
	CI           *CI        `json:"ci,omitempty"`
	SauceJob     *Job       `json:"sauce_job,omitempty"`
	Errors       []JobError `json:"errors,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
}

type CI struct {
	RefName    string `json:"ref_name,omitempty"`
	CommitSha  string `json:"commit_sha,omitempty"`
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
}

type Job struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type JobError struct {
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// The different states that a run can be in.
const (
	StatePassed  = "passed"
	StateFailed  = "failed"
	StateSkipped = "skipped"
)

type Service interface {
	GetHistory(context.Context, iam.User, config.LaunchOrder) (JobHistory, error)
	PostTestRun(ctx context.Context, runs []TestRun) error
}
