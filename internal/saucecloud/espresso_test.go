package saucecloud

import (
	"context"
	"github.com/jarcoal/httpmock"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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
	runner := &EspressoRunner{
		Project: espresso.Project{
			Espresso: espresso.Espresso{
				App:     "/path/to/app.apk",
				TestApp: "/path/to/testApp.apk",
			},
			Suites: []espresso.Suite{
				espresso.Suite{
					Name: "valid espresso project",
					Devices: []config.Device{
						config.Device{
							Name:             "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						config.Device{
							Name:             "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
			},
		},
	}
	assert.Equal(t, runner.calculateJobsCount(runner.Project.Suites), 3)
}

func TestEspressoRunner_RunProject(t *testing.T) {
	httpmock.Activate()
	defer func() {
		httpmock.DeactivateAndReset()
	}()
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
	writer := mocks.FakeJobWriter{
		UploadAssetFn: func(jobID string, fileName string, content []byte) error {
			return nil
		},
	}
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}
	runner := &EspressoRunner{
		CloudRunner: CloudRunner{
			JobStarter:      &starter,
			JobReader:       &reader,
			JobWriter:       &writer,
			CCYReader:       ccyReader,
			ProjectUploader: uploader,
		},
		Project: espresso.Project{
			Espresso: espresso.Espresso{
				App:     "/path/to/app.apk",
				TestApp: "/path/to/testApp.apk",
			},
			Suites: []espresso.Suite{
				espresso.Suite{
					Name: "my espresso project",
					Devices: []config.Device{
						config.Device{
							Name:             "Android GoogleApi Emulator",
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
}
