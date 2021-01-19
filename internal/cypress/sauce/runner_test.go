package sauce

import (
	"archive/zip"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	githubapi "github.com/google/go-github/v32/github"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
)

func TestPreliminarySteps_Basic(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	falseValue := false
	releases := []githubapi.RepositoryRelease{{Name: &v0, Prerelease: &falseValue}}
	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+cypress.RunnerGhOrg+"/"+cypress.RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	runner := Runner{Project: cypress.Project{Cypress: cypress.Cypress{Version: "5.6.2"}}}
	assert.Nil(t, runner.checkCypressVersionAvailability())
}

func TestPreliminarySteps_NoCypressVersion(t *testing.T) {
	want := "no cypress version provided"
	runner := Runner{}
	err := runner.checkCypressVersionAvailability()
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), want)
}

// Add support with latest
func TestPreliminarySteps_CypressLatest(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	falseValue := false
	releases := []githubapi.RepositoryRelease{{Name: &v0, Prerelease: &falseValue}}
	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+cypress.RunnerGhOrg+"/"+cypress.RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	wantVersion := v0
	runner := Runner{Project: cypress.Project{Cypress: cypress.Cypress{Version: "latest"}}}
	assert.Nil(t, runner.checkCypressVersionAvailability())
	assert.Equal(t, runner.Project.Cypress.Version, wantVersion)
}

// Add support with latest
func TestPreliminarySteps_CypressVersionNotAvailable(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	trueValue := true
	releases := []githubapi.RepositoryRelease{{Name: &v0, Prerelease: &trueValue}}
	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+cypress.RunnerGhOrg+"/"+cypress.RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	runner := Runner{Project: cypress.Project{
		Cypress: cypress.Cypress{Version: "5.6.3"},
		Docker:  config.Docker{Image: config.Image{Name: cypress.DefaultDockerImage}},
	}}
	assert.NotNil(t, runner.checkCypressVersionAvailability())
}

// Add support with latest
func TestPreliminarySteps_ErrorFetchingLatest(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+cypress.RunnerGhOrg+"/"+cypress.RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(400, map[string]string{})
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	runner := Runner{Project: cypress.Project{
		Cypress: cypress.Cypress{Version: "5.6.3"},
		Docker:  config.Docker{Image: config.Image{Name: cypress.DefaultDockerImage}},
	}}
	assert.NotNil(t, runner.checkCypressVersionAvailability())
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
	runner := Runner{
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
	runner := Runner{
		JobStarter: &starter,
		JobReader:  &reader,
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

	runner := Runner{
		Project: cypress.Project{
			Cypress: cypress.Cypress{
				ConfigFile:  "../../../tests/e2e/cypress.json",
				ProjectPath: "../../../tests/e2e/cypress/",
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
	runner := Runner{
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
