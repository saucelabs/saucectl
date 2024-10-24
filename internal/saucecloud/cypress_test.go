package saucecloud

import (
	"context"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/config"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
)

func TestRunSuite(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	runner := CypressRunner{
		CloudRunner: CloudRunner{
			JobService: JobService{
				VDCStarter: &mocks.FakeJobStarter{
					StartJobFn: func(context.Context, job.StartOptions) (jobID string, isRDC bool, err error) {
						return "fake-job-id", false, nil
					},
				},
				VDCReader: &mocks.FakeJobReader{
					PollJobFn: func(_ context.Context, id string, _ time.Duration, _ time.Duration) (job.Job, error) {
						return job.Job{ID: id, Passed: true}, nil
					},
				},
				VDCWriter: &mocks.FakeJobWriter{
					UploadAssetFn: func(string, string, string, []byte) error {
						return nil
					},
				},
			},
		},
	}

	opts := job.StartOptions{}
	j, skipped, err := runner.runJob(opts)
	assert.Nil(t, err)
	assert.False(t, skipped)
	assert.Equal(t, j.ID, "fake-job-id")
}

func TestRunSuites(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	runner := CypressRunner{
		CloudRunner: CloudRunner{
			JobService: JobService{
				VDCStarter: &mocks.FakeJobStarter{
					StartJobFn: func(context.Context, job.StartOptions) (jobID string, isRDC bool, err error) {
						return "fake-job-id", false, nil
					},
				},
				VDCReader: &mocks.FakeJobReader{
					PollJobFn: func(_ context.Context, id string, _ time.Duration, _ time.Duration) (job.Job, error) {
						return job.Job{ID: id, Passed: true, Status: job.StatePassed}, nil
					},
					GetJobAssetFileNamesFn: func(context.Context, string) ([]string, error) {
						return []string{"file1", "file2"}, nil
					},
					GetJobAssetFileContentFn: func(context.Context, string, string) ([]byte, error) {
						return []byte("file content"), nil
					},
				},
				VDCWriter: &mocks.FakeJobWriter{
					UploadAssetFn: func(string, string, string, []byte) error {
						return nil
					},
				},
				VDCDownloader: &mocks.FakeArtifactDownloader{
					DownloadArtifactFn: func(job.Job, bool) []string {
						return []string{}
					},
				},
			},
			BuildService: &mocks.FakeBuildReader{
				GetBuildIDFn: func(context.Context, string, build.Source) (string, error) {
					return "build-id", nil
				},
			},
			InsightsService: mocks.FakeInsightService{
				PostTestRunFn: func(context.Context, []insights.TestRun) error {
					return nil
				},
				ReadJobFn: func(context.Context, string) (job.Job, error) {
					return job.Job{}, nil
				},
			},
		},
		Project: &v1.Project{
			Suites: []v1.Suite{
				{Name: "dummy-suite"},
			},
			Sauce: config.SauceConfig{
				Concurrency: 1,
			},
		},
	}
	ret := runner.runSuites("dummy-id", []string{})
	assert.True(t, ret)
}
