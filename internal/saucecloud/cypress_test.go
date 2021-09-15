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

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
)

func TestPreliminarySteps_Basic(t *testing.T) {
	runner := CypressRunner{Project: cypress.Project{Cypress: cypress.Cypress{Version: "5.6.2"}}}
	assert.Nil(t, runner.checkCypressVersion())
}

func TestPreliminarySteps_NoCypressVersion(t *testing.T) {
	want := "missing cypress version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers"
	runner := CypressRunner{}
	err := runner.checkCypressVersion()
	assert.NotNil(t, err)
	assert.Equal(t, want, err.Error())
}

func TestRunSuite(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Fake JobStarter
	starter := mocks.FakeJobStarter{
		StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
			return "fake-job-id", false, nil
		},
	}
	reader := mocks.FakeJobReader{
		PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
			return job.Job{ID: id, Passed: true}, nil
		},
	}
	writer := mocks.FakeJobWriter{
		UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
			return nil
		},
	}
	runner := CypressRunner{
		CloudRunner: CloudRunner{
			JobStarter: &starter,
			JobReader:  &reader,
			JobWriter:  &writer,
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

	// Fake JobStarter
	starter := mocks.FakeJobStarter{
		StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
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
	downloader := &mocks.FakeArifactDownloader{
		DownloadArtifactFn: func(jobID string) {
		},
	}
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	runner := CypressRunner{
		CloudRunner: CloudRunner{
			JobStarter:         &starter,
			JobReader:          &reader,
			JobWriter:          &writer,
			CCYReader:          ccyReader,
			ArtifactDownloader: downloader,
		},
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
			RootDir: "../../tests/e2e/",
			Cypress: cypress.Cypress{
				ConfigFile: "cypress.json",
			},
		},
	}
	wd, _ := os.Getwd()
	log.Info().Msg(wd)

	z, err := runner.archiveProject(runner.Project, "./test-arch/", runner.Project.RootDir, "")
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
		CloudRunner: CloudRunner{
			ProjectUploader: uploader,
		},
	}
	id, err := runner.uploadProject("/my-dummy-project.zip", "project")
	assert.Equal(t, "fake-id", id)
	assert.Nil(t, err)

	uploader.UploadSuccess = false
	id, err = runner.uploadProject("/my-dummy-project.zip", "project")
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
		StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
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
	downloader := mocks.FakeArifactDownloader{
		DownloadArtifactFn: func(jobID string) {},
	}
	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}

	runner := CypressRunner{
		CloudRunner: CloudRunner{
			JobStarter:         &starter,
			JobReader:          &reader,
			JobWriter:          &writer,
			CCYReader:          ccyReader,
			ProjectUploader:    uploader,
			ArtifactDownloader: &downloader,
		},
		Project: cypress.Project{
			RootDir: ".",
			Cypress: cypress.Cypress{
				Version:    "5.6.0",
				ConfigFile: "../../tests/e2e/cypress.json",
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

func TestCypress_GetSuiteNames(t *testing.T) {
	runner := &CypressRunner{
		Project: cypress.Project{
			Suites: []cypress.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, "suite1, suite2, suite3", runner.getSuiteNames())
}
