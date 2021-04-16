package saucecloud

import (
	"context"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/playwright"

	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/stretchr/testify/assert"
)

func TestTestcafe_GetSuiteNames(t *testing.T) {
	runner := &TestcafeRunner{
		Project: testcafe.Project{
			Suites: []testcafe.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, "suite1, suite2, suite3", runner.getSuiteNames())
}

func TestRunSuites_TestCafe_NoConcurrency(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Fake JobStarter
	starter := mocks.FakeJobStarter{
		StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, err error) {
			return "fake-job-id", nil
		},
	}
	reader := mocks.FakeJobReader{
		PollJobFn: func(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
			return job.Job{ID: id, Passed: true}, nil
		},
	}
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 0, nil
	}}
	runner := PlaywrightRunner{
		CloudRunner: CloudRunner{
			JobStarter: &starter,
			JobReader:  &reader,
			CCYReader:  ccyReader,
		},
		Project: playwright.Project{
			Suites: []playwright.Suite{
				{Name: "dummy-suite"},
			},
			Sauce: config.SauceConfig{
				Concurrency: 1,
			},
		},
	}
	ret := runner.runSuites("dummy-file-id")
	assert.False(t, ret)
}

func Test_calcTestcafeJobsCount(t *testing.T) {
	testCases := []struct {
		name              string
		suites            []testcafe.Suite
		expectedJobsCount int
	}{
		{
			name: "single suite",
			suites: []testcafe.Suite{
				{
					Name: "single suite",
				},
			},
			expectedJobsCount: 1,
		},
		{
			name: "two suites",
			suites: []testcafe.Suite{
				{
					Name: "first suite",
				},
				{
					Name: "second suite",
				},
			},
			expectedJobsCount: 2,
		},
		{
			name: "suites with devices and platfrom versions",
			suites: []testcafe.Suite{
				{
					Name: "first suite",
				},
				{
					Name: "second suite",
				},
				{
					Name: "suite with one device and two platforms",
					Devices: []config.Device{
						{PlatformVersions: []string{"12.0", "14.3"}},
					},
				},
				{
					Name: "suite with two device and two platforms",
					Devices: []config.Device{
						{PlatformVersions: []string{"12.0", "14.3"}},
						{PlatformVersions: []string{"12.0", "14.3"}},
					},
				},
			},
			expectedJobsCount: 8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tr := TestcafeRunner{}
			got := tr.calcTestcafeJobsCount(tc.suites)
			if tc.expectedJobsCount != got {
				t.Errorf("expected: %d, got: %d", tc.expectedJobsCount, got)
			}
		})
	}
}
