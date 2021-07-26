package saucecloud

import (
	"context"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
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
					TestOptions: espresso.TestOptions{
						NumShards: 3,
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
					TestOptions: espresso.TestOptions{
						NumShards: 3,
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
	// Fake JobStarter
	var startOpts job.StartOptions
	starter := mocks.FakeJobStarter{
		StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
			startOpts = opts
			return "fake-job-id", false, nil
		},
	}
	reader := mocks.FakeJobReader{
		PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
			return job.Job{ID: id, Passed: true}, nil
		},
		GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
			return []string{"file1", "file2"}, nil
		},
		GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
			return []byte("file content"), nil
		},
	}

	writer := mocks.FakeJobWriter{
		UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
			return nil
		},
	}
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}
	downloader := mocks.FakeArifactDownloader{
		DownloadArtifactFn: func(jobID string) {},
	}

	runner := &EspressoRunner{
		CloudRunner: CloudRunner{
			JobStarter:         &starter,
			JobReader:          &reader,
			JobWriter:          &writer,
			CCYReader:          ccyReader,
			ProjectUploader:    uploader,
			ArtifactDownloader: &downloader,
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
}
