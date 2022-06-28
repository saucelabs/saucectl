package saucecloud

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/config"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/framework"
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
			CCYReader: mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
				return 1, nil
			}},
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
	ret := runner.runSuites("dummy-file-id")
	assert.True(t, ret)
}

func TestArchiveProject(t *testing.T) {
	err := os.Mkdir("./test-arch/", 0755)
	if err != nil {
		t.Errorf("failed to create test-arch directory: %v", err)
		return
	}
	defer func() {
		os.RemoveAll("./test-arch/")
	}()

	runner := CypressRunner{
		Project: &v1.Project{
			RootDir: "../../tests/e2e/",
			Cypress: v1.Cypress{
				ConfigFile: "cypress.json",
			},
		},
	}
	wd, _ := os.Getwd()
	log.Info().Msg(wd)

	z, err := runner.archiveFolder(runner.Project, "./test-arch/", runner.Project.GetRootDir(), "")
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
	if err != nil && err != io.EOF {
		t.Errorf("error reading cypress.json: %s", err)
	}
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
	assert.Equal(t, "storage:fake-id", id)
	assert.Nil(t, err)

	uploader.UploadSuccess = false
	id, err = runner.uploadProject("/my-dummy-project.zip", "project")
	assert.Equal(t, "", id)
	assert.NotNil(t, err)

}

func TestRunProject(t *testing.T) {
	err := os.Mkdir("./test-arch/", 0755)
	if err != nil {
		t.Errorf("failed to create test-arch directory: %v", err)
	}
	httpmock.Activate()
	defer func() {
		os.RemoveAll("./test-arch/")
		httpmock.DeactivateAndReset()
	}()

	ccyReader := mocks.CCYReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
		return 1, nil
	}}
	uploader := &mocks.FakeProjectUploader{
		UploadSuccess: true,
	}

	mdService := &mocks.FakeFrameworkInfoReader{
		VersionsFn: func(ctx context.Context, frameworkName string) ([]framework.Metadata, error) {
			return []framework.Metadata{{
				FrameworkName:    "cypress",
				FrameworkVersion: "5.6.0",
			}}, nil
		},
	}

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
					DownloadArtifactFn: func(jobID, suiteName string) []string {
						return []string{}
					},
				},
			},
			CCYReader:              ccyReader,
			ProjectUploader:        uploader,
			MetadataService:        mdService,
			MetadataSearchStrategy: framework.ExactStrategy{},
		},
		Project: &v1.Project{
			RootDir: ".",
			Cypress: v1.Cypress{
				Version:    "5.6.0",
				ConfigFile: "../../tests/e2e/cypress.json",
			},
			Suites: []v1.Suite{
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
