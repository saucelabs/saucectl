package spotlight

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/report"
)

func ExampleReporter_Render() {
	startTime := time.Now()
	restResults := []report.TestResult{
		{
			Name:      "Chrome",
			Duration:  171452 * time.Millisecond,
			StartTime: startTime,
			EndTime:   startTime.Add(171452 * time.Millisecond),
			Status:    job.StateFailed,
			Browser:   "Chrome",
			Platform:  "Windows 10",
			URL:       "https://app.saucelabs.com/tests/1234567890abcdef",
			Attempts: []report.Attempt{
				{
					Status: job.StateFailed,
					TestSuites: junit.TestSuites{
						TestSuites: []junit.TestSuite{
							{
								Name: "",
								TestCases: []junit.TestCase{
									{
										Name:      "TestCase1",
										ClassName: "com.saucelabs.examples.SauceTest",
										Error: &junit.Error{
											Message: "Whoops!",
											Type:    "AssertionError",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	r := Reporter{
		Dst: os.Stdout,
	}

	for _, tr := range restResults {
		r.Add(tr)
	}

	r.Render()
	// Output:
	//Spotlight:
	//
	//  ✖ Chrome
	//    ● URL: https://app.saucelabs.com/tests/1234567890abcdef
	//    ● Failed Tests: (showing max. 5)
	//      ✖ com.saucelabs.examples.SauceTest › TestCase1
}

func TestReporter_Add(t *testing.T) {
	type fields struct {
		TestResults []report.TestResult
		Dst         io.Writer
	}
	type args struct {
		t report.TestResult
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "skip passed tests",
			args: args{
				t: report.TestResult{
					Status: job.StatePassed,
				},
			},
			want: 0,
		},
		{
			name: "skipped in-progress tests",
			args: args{
				t: report.TestResult{
					Status: job.StateInProgress,
				},
			},
			want: 0,
		},
		{
			name: "include failed tests",
			args: args{
				t: report.TestResult{
					Status: job.StateFailed,
				},
			},
			want: 1,
		},
		{
			name: "include errored tests",
			args: args{
				t: report.TestResult{
					Status: job.StateError,
				},
			},
			want: 1,
		},
		{
			name: "include timed out tests",
			args: args{
				t: report.TestResult{
					TimedOut: true,
				},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reporter{}
			r.Add(tt.args.t)
			if added := len(r.TestResults); added != tt.want {
				t.Errorf("Reporter.Add() added %d results, want %d", added, tt.want)
			}
		})
	}
}
