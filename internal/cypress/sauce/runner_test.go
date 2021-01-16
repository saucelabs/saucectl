package sauce


import (
	githubapi "github.com/google/go-github/v32/github"
	"github.com/jarcoal/httpmock"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestPreliminarySteps_Basic(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	falseValue := false
	releases := []githubapi.RepositoryRelease{{ Name:  &v0, Prerelease: &falseValue }}
	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+cypress.RunnerGhOrg+"/"+cypress.RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	runner := Runner{Project: cypress.Project{ Cypress: cypress.Cypress{ Version: "5.6.2" }}}
	assert.Nil(t, runner.preliminarySteps())
}

func TestPreliminarySteps_NoCypressVersion(t *testing.T) {
	want := "no cypress version provided"
	runner := Runner{}
	err := runner.preliminarySteps()
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), want)
}

// Add support with latest
func TestPreliminarySteps_CypressLatest(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	falseValue := false
	releases := []githubapi.RepositoryRelease{{ Name:  &v0, Prerelease: &falseValue }}
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
	runner := Runner{Project: cypress.Project{ Cypress: cypress.Cypress{ Version: "latest" }}}
	assert.Nil(t, runner.preliminarySteps())
	assert.Equal(t, runner.Project.Cypress.Version, wantVersion)
}

// Add support with latest
func TestPreliminarySteps_CypressVersionNotAvailable(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	trueValue := true
	releases := []githubapi.RepositoryRelease{{ Name:  &v0, Prerelease: &trueValue }}
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
		Cypress: cypress.Cypress{ Version: "5.6.3" },
		Docker: config.Docker{ Image: config.Image{ Name: cypress.DefaultDockerImage }},
	}}
	assert.NotNil(t, runner.preliminarySteps())
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
		Cypress: cypress.Cypress{ Version: "5.6.3" },
		Docker: config.Docker{ Image: config.Image{ Name: cypress.DefaultDockerImage }},
	}}
	assert.NotNil(t, runner.preliminarySteps())
}