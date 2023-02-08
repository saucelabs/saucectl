package saucecloud

import (
	"context"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	j "github.com/saucelabs/saucectl/internal/cmd/jobs/job"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestEspresso_GetSuiteNames(t *testing.T) {
	runner := &EspressoRunner{
		Project: espresso.Project{
			Suites: []espresso.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, "suite1, suite2, suite3", runner.getSuiteNames())
}

func TestEspressoRunner_CalculateJobCount(t *testing.T) {
	tests := []struct {
		name   string
		suites []espresso.Suite
		wants  int
	}{
		{
			name: "should multiply emulator combinations",
			suites: []espresso.Suite{
				{
					Name: "valid espresso project",
					Emulators: []config.Emulator{
						{
							Name:             "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						{
							Name:             "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
			},
			wants: 3,
		},
		{
			name:  "should multiply jobs by NumShards if defined",
			wants: 18,
			suites: []espresso.Suite{
				{
					Name: "first suite",
					TestOptions: map[string]interface{}{
						"numShards": 3,
					},
					Emulators: []config.Emulator{
						{
							Name:             "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						{
							Name:             "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
				{
					Name: "second suite",
					TestOptions: map[string]interface{}{
						"numShards": 3,
					},
					Emulators: []config.Emulator{
						{
							Name:             "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						{
							Name:             "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		runner := &EspressoRunner{
			Project: espresso.Project{
				Espresso: espresso.Espresso{
					App:     "/path/to/app.apk",
					TestApp: "/path/to/testApp.apk",
				},
				Suites: tt.suites,
			},
		}

		assert.Equal(t, runner.calculateJobsCount(runner.Project.Suites), tt.wants)
	}
}

func TestEspressoRunner_RunProject(t *testing.T) {
	httpmock.Activate()
	defer func() {
		httpmock.DeactivateAndReset()
	}()

	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}

	var startOpts job.StartOptions
	runner := &EspressoRunner{
		CloudRunner: CloudRunner{
			JobService: JobService{
				VDCStarter: &mocks.FakeJobStarter{
					StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
						startOpts = opts
						return "fake-job-id", false, nil
					},
				},
				VDCReader: &mocks.FakeJobReader{
					PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
						return job.Job{ID: id, Passed: true}, nil
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
					DownloadArtifactFn: func(jobID, suiteName string) []string { return []string{} },
				},
			},
			CCYReader:       ccyReader,
			ProjectUploader: uploader,
			InsightsService: mocks.FakeInsightService{
				PostTestRunFn: func(ctx context.Context, runs []insights.TestRun) error {
					return nil
				},
				ReadJobFn: func(ctx context.Context, id string) (j.Job, error) {
					return j.Job{}, nil
				},
			},
		},
		Project: espresso.Project{
			Espresso: espresso.Espresso{
				App:     "/path/to/app.apk",
				TestApp: "/path/to/testApp.apk",
			},
			Suites: []espresso.Suite{
				{
					Name: "my espresso project",
					Emulators: []config.Emulator{
						{
							Name:             "Android GoogleApi Emulator",
							Orientation:      "landscape",
							PlatformVersions: []string{"11.0"},
						},
					},
					TestOptions: map[string]interface{}{
						"notAnnotation": "my.annotation",
					},
				},
			},
			Sauce: config.SauceConfig{
				Concurrency: 1,
			},
		},
	}
	cnt, err := runner.RunProject()
	assert.Nil(t, err)
	assert.Equal(t, cnt, 0)
	assert.Equal(t, "landscape", startOpts.DeviceOrientation)
	assert.Equal(t, "my.annotation", startOpts.TestOptions["notAnnotation"])
}
