package saucecloud

import (
	"context"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/build"
	j "github.com/saucelabs/saucectl/internal/cmd/jobs/job"
	"github.com/saucelabs/saucectl/internal/config"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
)

func TestPreliminarySteps_Basic(t *testing.T) {
	runner := CypressRunner{Project: &v1.Project{Cypress: v1.Cypress{Version: "10.1.0"}}}
	assert.Nil(t, runner.checkCypressVersion())
}

func TestPreliminarySteps_NoCypressVersion(t *testing.T) {
	want := "missing cypress version. Check available versions here: https://docs.saucelabs.com/dev/cli/saucectl/#supported-frameworks-and-browsers"
	runner := CypressRunner{
		Project: &v1.Project{},
	}
	err := runner.checkCypressVersion()
	assert.NotNil(t, err)
	assert.Equal(t, want, err.Error())
}

func TestRunSuite(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	runner := CypressRunner{
		CloudRunner: CloudRunner{
			JobService: JobService{
				VDCStarter: &mocks.FakeJobStarter{
					StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
						return "fake-job-id", false, nil
					},
				},
				VDCReader: &mocks.FakeJobReader{
					PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
						return job.Job{ID: id, Passed: true}, nil
					},
				},
				VDCWriter: &mocks.FakeJobWriter{
					UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
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
					StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
						return "fake-job-id", false, nil
					},
				},
				VDCReader: &mocks.FakeJobReader{
					PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
						return job.Job{ID: id, Passed: true, Status: job.StatePassed}, nil
					},
					GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
						return []string{"file1", "file2"}, nil
					},
					GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
						return []byte("file content"), nil
					},
				},
				VDCWriter: &mocks.FakeJobWriter{
					UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
						return nil
					},
				},
				VDCDownloader: &mocks.FakeArtifactDownloader{
					DownloadArtifactFn: func(jobID string, suiteName string) []string {
						return []string{}
					},
				},
			},
			BuildService: &mocks.FakeBuildReader{
				GetBuildIDFn: func(ctx context.Context, jobID string, buildSource build.Source) (string, error) {
					return "build-id", nil
				},
			},
			InsightsService: mocks.FakeInsightService{
				PostTestRunFn: func(ctx context.Context, runs []insights.TestRun) error {
					return nil
				},
				ReadJobFn: func(ctx context.Context, id string) (j.Job, error) {
					return j.Job{}, nil
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
	ret := runner.runSuites(map[uploadType]string{projectUpload: "dummy-file-id"})
	assert.True(t, ret)
}
