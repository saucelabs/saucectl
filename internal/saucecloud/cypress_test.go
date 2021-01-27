package saucecloud

import (
	"archive/zip"
	"context"
	"os"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
)

func TestPreliminarySteps_Basic(t *testing.T) {
	runner := CypressRunner{Project: cypress.Project{Cypress: cypress.Cypress{Version: "5.6.2"}}}
	assert.Nil(t, runner.checkCypressVersion())
}

func TestPreliminarySteps_NoCypressVersion(t *testing.T) {
	want := "Missing cypress version. Check out available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers"
	runner := CypressRunner{}
	err := runner.checkCypressVersion()
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), want)
}

func TestRunSuite(t *testing.T) {
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
			time.Sleep(5 * time.Second)
			return job.Job{ID: id, Passed: true}, nil
		},
	}
	runner := CypressRunner{
		JobStarter: &starter,
		JobReader:  &reader,
	}
	suite := cypress.Suite{}
	fileID := "dummy-file-id"
	j, err := runner.runSuite(suite, fileID)
	assert.Nil(t, err)
	assert.Equal(t, j.ID, "fake-job-id")
}

func TestRunSuites(t *testing.T) {
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
			time.Sleep(5 * time.Second)
			return job.Job{ID: id, Passed: true}, nil
		},
	}
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	runner := CypressRunner{
		JobStarter: &starter,
		JobReader:  &reader,
		CCYReader:  ccyReader,
		Project: cypress.Project{
			Suites: []cypress.Suite{
				{Name: "dummy-suite"},
			},
			Sauce: config.SauceConfig{
				Concurrency: 1,
			},
		},
	}
	ret := runner.runSuites("dummy-file-id")
	assert.True(t, ret)
}

func TestArchiveProject(t *testing.T) {
	os.Mkdir("./test-arch/", 0755)
	defer func() {
		os.RemoveAll("./test-arch/")
	}()

	runner := CypressRunner{
		Project: cypress.Project{
			Cypress: cypress.Cypress{
				ConfigFile:  "../../tests/e2e/cypress.json",
				ProjectPath: "../../tests/e2e/cypress/",
			},
		},
	}
	wd, _ := os.Getwd()
	log.Info().Msg(wd)
	z, err := runner.archiveProject("./test-arch/")
	if err != nil {
		t.Fail()
	}
	zipFile, _ := os.Open(z)
	defer func() {
		zipFile.Close()
	}()
	zipInfo, _ := zipFile.Stat()
	zipStream, _ := zip.NewReader(zipFile, zipInfo.Size())

	var cypressConfig *zip.File
	for _, f := range zipStream.File {
		if f.Name == "cypress.json" {
			cypressConfig = f
			break
		}
	}
	assert.NotNil(t, cypressConfig)
	rd, _ := cypressConfig.Open()
	b := make([]byte, 3)
	n, err := rd.Read(b)
	assert.Equal(t, 3, n)
	assert.Equal(t, []byte("{}\n"), b)
}

func TestUploadProject(t *testing.T) {
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}
	runner := CypressRunner{
		ProjectUploader: uploader,
	}
	id, err := runner.uploadProject("/my-dummy-project.zip")
	assert.Equal(t, "fake-id", id)
	assert.Nil(t, err)

	uploader.UploadSuccess = false
	id, err = runner.uploadProject("/my-dummy-project.zip")
	assert.Equal(t, "", id)
	assert.NotNil(t, err)

}

func TestRunProject(t *testing.T) {
	os.Mkdir("./test-arch/", 0755)
	httpmock.Activate()
	defer func() {
		os.RemoveAll("./test-arch/")
		httpmock.DeactivateAndReset()
	}()

	// Fake JobStarter
	starter := mocks.FakeJobStarter{
		CheckFrameworkAvailabilitySuccess: true,
		StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, err error) {
			return "fake-job-id", nil
		},
	}
	reader := mocks.FakeJobReader{
		PollJobFn: func(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
			time.Sleep(5 * time.Second)
			return job.Job{ID: id, Passed: true}, nil
		},
	}
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}
	runner := CypressRunner{
		JobStarter:      &starter,
		JobReader:       &reader,
		CCYReader:       ccyReader,
		ProjectUploader: uploader,
		Project: cypress.Project{
			Cypress: cypress.Cypress{
				Version:     "5.6.0",
				ConfigFile:  "../../tests/e2e/cypress.json",
				ProjectPath: "../../tests/e2e/cypress/",
			},
			Suites: []cypress.Suite{
				{Name: "dummy-suite"},
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

func TestLogSuiteConsole(t *testing.T) {
	reader := &mocks.FakeJobReader{
		GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
			return []byte("dummy-content"), nil
		},
	}
	runner := CypressRunner{
		JobReader: reader,
	}
	res := result{
		job: job.Job{
			ID: "fake-job-id",
		},
	}
	runner.logSuiteConsole(res)
}
