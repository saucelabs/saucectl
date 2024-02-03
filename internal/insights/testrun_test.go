package insights

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/saucereport"
)

func Test_uniformizeJSONStatus(t *testing.T) {
	// Unsetting Github env vars
	os.Unsetenv("GITHUB_SERVER_URL")
	os.Unsetenv("GITHUB_REPOSITORY")
	os.Unsetenv("GITHUB_RUN_ID")
	os.Unsetenv("GITHUB_REF_NAME")
	os.Unsetenv("GITHUB_SHA")
	os.Unsetenv("GITHUB_ACTOR")

	type args struct {
		status string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Success",
			args: args{
				status: "success",
			},
			want: StatePassed,
		},
		{
			name: "Failed",
			args: args{
				status: "failed",
			},
			want: StateFailed,
		},
		{
			name: "Errored",
			args: args{
				status: "error",
			},
			want: StateFailed,
		},
		{
			name: "Skipped",
			args: args{
				status: "skipped",
			},
			want: StateSkipped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniformizeJSONStatus(tt.args.status); got != tt.want {
				t.Errorf("uniformizeJSONStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromSauceReport(t *testing.T) {
	// Unsetting Github env vars
	os.Unsetenv("GITHUB_SERVER_URL")
	os.Unsetenv("GITHUB_REPOSITORY")
	os.Unsetenv("GITHUB_RUN_ID")
	os.Unsetenv("GITHUB_REF_NAME")
	os.Unsetenv("GITHUB_SHA")
	os.Unsetenv("GITHUB_ACTOR")

	type args struct {
		report saucereport.SauceReport
	}
	tests := []struct {
		name string
		args args
		want []TestRun
	}{
		{
			name: "Basic Passing Report",
			args: args{
				report: saucereport.SauceReport{
					Status: StatePassed,
					Suites: []saucereport.Suite{
						{
							Name: "Suite #1",
							Tests: []saucereport.Test{
								{
									Name:      "Test #1.1",
									Status:    StatePassed,
									StartTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
									Duration:  20,
								},
								{
									Name:      "Test #1.2",
									Status:    StateFailed,
									StartTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
									Duration:  20,
									Output:    "my-dummy-failure",
								},
							},
						},
					},
				},
			},
			want: []TestRun{
				{
					Name:         "Test #1.1",
					CreationTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					StartTime:    time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					EndTime:      time.Date(2022, 12, 13, 14, 15, 36, 17, time.UTC),
					Duration:     20,
					Status:       StatePassed,
					SauceJob: &Job{
						Name: "jobName",
						ID:   "jobID",
					},
					Type:     TypeWeb,
					Platform: job.SourceVDC,
				},
				{
					Name:         "Test #1.2",
					CreationTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					StartTime:    time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					EndTime:      time.Date(2022, 12, 13, 14, 15, 36, 17, time.UTC),
					Duration:     20,
					Status:       StateFailed,
					Errors: []TestRunError{
						{
							Message: "my-dummy-failure",
						},
					},
					SauceJob: &Job{
						Name: "jobName",
						ID:   "jobID",
					},
					Type:     TypeWeb,
					Platform: job.SourceVDC,
				},
			},
		},
		{
			name: "Nested suites",
			args: args{
				report: saucereport.SauceReport{
					Suites: []saucereport.Suite{
						{
							Name:   "Suite #1",
							Status: StatePassed,
							Suites: []saucereport.Suite{
								{
									Name: "Suite #1.1",
									Tests: []saucereport.Test{
										{
											Name:      "Test #1.1.1",
											Status:    StatePassed,
											StartTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
											Duration:  20,
										},
										{
											Name:      "Test #1.1.2",
											Status:    StatePassed,
											StartTime: time.Date(2022, 12, 14, 14, 15, 16, 17, time.UTC),
											Duration:  20,
										},
									},
								},
							},
							Tests: []saucereport.Test{
								{
									Name:      "Test #1.1",
									Status:    StatePassed,
									StartTime: time.Date(2022, 12, 15, 14, 15, 16, 17, time.UTC),
									Duration:  20,
								},
							},
						},
					},
				},
			},
			want: []TestRun{
				{
					Name:         "Test #1.1",
					CreationTime: time.Date(2022, 12, 15, 14, 15, 16, 17, time.UTC),
					StartTime:    time.Date(2022, 12, 15, 14, 15, 16, 17, time.UTC),
					EndTime:      time.Date(2022, 12, 15, 14, 15, 36, 17, time.UTC),
					Duration:     20,
					Status:       StatePassed,
					SauceJob: &Job{
						Name: "jobName",
						ID:   "jobID",
					},
					Type:     TypeWeb,
					Platform: job.SourceVDC,
				},
				{
					Name:         "Test #1.1.1",
					CreationTime: time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					StartTime:    time.Date(2022, 12, 13, 14, 15, 16, 17, time.UTC),
					EndTime:      time.Date(2022, 12, 13, 14, 15, 36, 17, time.UTC),
					Duration:     20,
					Status:       StatePassed,
					SauceJob: &Job{
						Name: "jobName",
						ID:   "jobID",
					},
					Type:     TypeWeb,
					Platform: job.SourceVDC,
				},
				{
					Name:         "Test #1.1.2",
					CreationTime: time.Date(2022, 12, 14, 14, 15, 16, 17, time.UTC),
					StartTime:    time.Date(2022, 12, 14, 14, 15, 16, 17, time.UTC),
					EndTime:      time.Date(2022, 12, 14, 14, 15, 36, 17, time.UTC),
					Duration:     20,
					Status:       StatePassed,
					SauceJob: &Job{
						Name: "jobName",
						ID:   "jobID",
					},
					Type:     TypeWeb,
					Platform: job.SourceVDC,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromSauceReport(tt.args.report, "jobID", "jobName", Details{}, false)

			// Replicate IDs as they are random
			for idx := range got {
				tt.want[idx].ID = got[idx].ID
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromSauceReport() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromJUnit(t *testing.T) {
	type args struct {
		suites junit.TestSuites
	}
	tests := []struct {
		name string
		args args
		want []TestRun
	}{
		{
			name: "Basic test",
			args: args{
				suites: junit.TestSuites{
					TestSuites: []junit.TestSuite{
						{
							Name:   "Test #1",
							Tests:  2,
							Errors: 1,
							Time:   "10.45",
							TestCases: []junit.TestCase{
								{
									Name:      "Test #1.1",
									Status:    StatePassed,
									Time:      "5.40",
									Timestamp: "2022-12-12T01:01:01Z",
									ClassName: "ClassName",
								},
								{
									Name:      "Test #1.2",
									Status:    StateFailed,
									Time:      "5.05",
									Timestamp: "2022-12-13T01:01:01Z",
									ClassName: "ClassName",
									Failure:   &junit.Failure{Message: "dummy-error-message"},
								},
							},
						},
					},
				},
			},
			want: []TestRun{
				{
					Name:         "ClassName.Test #1.1",
					Status:       StatePassed,
					CreationTime: time.Date(2022, 12, 12, 1, 1, 1, 0, time.UTC),
					StartTime:    time.Date(2022, 12, 12, 1, 1, 1, 0, time.UTC),
					Duration:     5,
					EndTime:      time.Date(2022, 12, 12, 1, 1, 6, 0, time.UTC),
					SauceJob: &Job{
						Name: "jobName",
						ID:   "jobID",
					},
					Type:     TypeWeb,
					Platform: job.SourceVDC,
				},
				{
					Name:         "ClassName.Test #1.2",
					Status:       StateFailed,
					CreationTime: time.Date(2022, 12, 13, 1, 1, 1, 0, time.UTC),
					StartTime:    time.Date(2022, 12, 13, 1, 1, 1, 0, time.UTC),
					Duration:     5,
					EndTime:      time.Date(2022, 12, 13, 1, 1, 6, 0, time.UTC),
					Errors: []TestRunError{
						{
							Message: "dummy-error-message",
						},
					},
					SauceJob: &Job{
						Name: "jobName",
						ID:   "jobID",
					},
					Type:     TypeWeb,
					Platform: job.SourceVDC,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromJUnit(tt.args.suites, "jobID", "jobName", Details{}, false)

			// Replicate IDs as they are random
			for idx := range got {
				tt.want[idx].ID = got[idx].ID
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromJUnit() got = %v, want %v", got, tt.want)
			}
		})
	}
}
