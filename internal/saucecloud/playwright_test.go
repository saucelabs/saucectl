package saucecloud

import (
	"context"
	"github.com/jarcoal/httpmock"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/stretchr/testify/assert"
)

func TestPlaywright_GetSuiteNames(t *testing.T) {
	runner := &PlaywrightRunner{
		Project: playwright.Project{
			Suites: []playwright.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, "suite1, suite2, suite3", runner.getSuiteNames())
}

func TestRunSuites_Playwright_NoConcurrency(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Fake JobStarter
	starter := mocks.FakeJobStarter{
		StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
			return "fake-job-id", false, nil
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