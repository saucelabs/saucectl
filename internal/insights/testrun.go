package insights

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xtgo/uuid"

	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/saucereport"
)

// TestRun represents a
type TestRun struct {
	ID           string         `json:"id,omitempty"`
	Name         string         `json:"name,omitempty"`
	UserID       string         `json:"user_id,omitempty"`
	OrgID        string         `json:"org_id,omitempty"`
	TeamID       string         `json:"team_id,omitempty"`
	GroupID      string         `json:"group_id,omitempty"`
	AuthorID     string         `json:"author_id,omitempty"`
	PathName     string         `json:"path_name,omitempty"`
	BuildID      string         `json:"build_id,omitempty"`
	BuildName    string         `json:"build_name,omitempty"`
	CreationTime time.Time      `json:"creation_time,omitempty"`
	StartTime    time.Time      `json:"start_time,omitempty"`
	EndTime      time.Time      `json:"end_time,omitempty"`
	Duration     int            `json:"duration,omitempty"`
	Browser      string         `json:"browser,omitempty"`
	Device       string         `json:"device,omitempty"`
	OS           string         `json:"os,omitempty"`
	AppName      string         `json:"app_name,omitempty"`
	Status       string         `json:"status,omitempty"`
	Platform     string         `json:"platform,omitempty"`
	Type         string         `json:"type,omitempty"`
	Framework    string         `json:"framework,omitempty"`
	CI           *CI            `json:"ci,omitempty"`
	SauceJob     *Job           `json:"sauce_job,omitempty"`
	Errors       []TestRunError `json:"errors,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
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

type TestRunError struct {
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

// The different types that a run can be.
const (
	TypeWeb    = "web"
	TypeMobile = "mobile"
	TypeAPI    = "api"
	TypeOther  = "other"
)

// The different platform that a run can be executed on.
const (
	PlatformVDC   = "vdc"
	PlatformRDC   = "rdc"
	PlatformAPI   = "api"
	PlatformOther = "other"
)

func FromJUnit(suites junit.TestSuites) ([]TestRun, error) {
	var testRuns []TestRun

	for _, s := range suites.TestSuites {
		for _, ss := range s.TestCases {
			status := StatePassed
			var startDate time.Time
			var err error
			if ss.Timestamp == "" {
				startDate, err = time.Parse(time.RFC3339, ss.Timestamp)
				if err != nil {
					log.Warn().Err(err).Msg("unable to parse date. using time.Now().")
					startDate = time.Now()
				}
			}
			duration, err := strconv.ParseFloat(ss.Time, 64)
			if err != nil {
				log.Warn().Err(err).Msg("unable to parse duration. using 0.")
			}
			endTime := startDate.Add(time.Duration(duration) * time.Second)
			var failures []TestRunError
			if ss.Failure != "" {
				status = StateFailed
				failures = []TestRunError{
					{
						Message: ss.Failure,
					},
				}
			}
			testRuns = append(testRuns, TestRun{
				Name:         fmt.Sprintf("%s.%s", ss.ClassName, ss.Name),
				ID:           uuid.NewRandom().String(),
				Status:       status,
				CreationTime: startDate,
				StartTime:    startDate,
				EndTime:      endTime,
				Duration:     int(duration),
				Errors:       failures,
			})
		}
	}

	return testRuns, nil
}

func FromSauceReport(report saucereport.SauceReport) ([]TestRun, error) {
	var testRuns []TestRun
	for _, s := range report.Suites {
		testRuns = append(testRuns, deepConvert(s)...)
	}
	return testRuns, nil
}

func deepConvert(suite saucereport.Suite) []TestRun {
	var runs []TestRun

	for _, test := range suite.Tests {
		newRun := TestRun{
			Name:         test.Name,
			ID:           uuid.NewRandom().String(),
			Status:       uniformizeJSONStatus(test.Status),
			CreationTime: test.StartTime,
			StartTime:    test.StartTime,
			EndTime:      test.StartTime.Add(time.Duration(test.Duration) * time.Second),
			Duration:     test.Duration,
		}
		if test.Status == StateFailed && test.Output != "" {
			newRun.Errors = []TestRunError{
				{
					Message: test.Output,
				},
			}
		}
		runs = append(runs, newRun)
	}

	for _, child := range suite.Suites {
		runs = append(runs, deepConvert(child)...)
	}
	return runs
}

func uniformizeJSONStatus(status string) string {
	switch status {
	case "failed":
		fallthrough
	case "error":
		return StateFailed
	case "skipped":
		return StateSkipped
	}
	return StatePassed
}