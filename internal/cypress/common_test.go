package cypress

import (
	githubapi "github.com/google/go-github/v32/github"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestStandardizeVersionFormat(t *testing.T) {
	assert.Equal(t, StandardizeVersionFormat("v5.6.0"), "5.6.0")
	assert.Equal(t, StandardizeVersionFormat("5.6.0"), "5.6.0")
	assert.Equal(t, StandardizeVersionFormat("v0.6.0"), "0.6.0")
}

func buildReleasesPayload() []githubapi.RepositoryRelease{
	v0 := "v5.6.2"
	v1 := "v5.6.1"
	falseValue := false
	trueValue := true
	return []githubapi.RepositoryRelease{
		{Name: &v0, Prerelease: &trueValue},
		{Name: &v1, Prerelease: &falseValue},
	}
}

func TestIsCypressVersionAvailable(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	releases := buildReleasesPayload()
	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+RunnerGhOrg+"/"+RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	tests := []struct{
		version string
		wantErr bool
		wantCloud bool
		wantDocker bool
	}{
		{"v5.6.2", false, false, true},
		{"5.6.2", false, false, true},
		{"v5.6.1", false, true, true},
		{"5.6.1", false, true, true},
		{"v5.6.0", false, false, false},
		{"5.6.0", false, false, false},
	}

	for _, tt := range tests {
		docker, cloud, err := IsCypressVersionAvailable(tt.version)
		assert.Equal(t, tt.wantDocker, docker, "docker output value mismatch")
		assert.Equal(t, tt.wantCloud, cloud, "cloud output value mismatch")
		if tt.wantErr {
			assert.NotNil(t, err, "error is expected")
		} else {
			assert.Nil(t, err, "error is not expected")
		}
	}
}

func TestGetLatestVersionAll(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	releases := buildReleasesPayload()
	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+RunnerGhOrg+"/"+RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	version, err := GetLatestCloudVersion()
	assert.Nil(t, err, "no error should occurred")
	assert.Equal(t, "5.6.1", version, "version is not expected")

	version, err = GetLatestDockerVersion()
	assert.Nil(t, err, "no error should occurred")
	assert.Equal(t, "5.6.2", version, "version is not expected")

}


func TestGetLatestVersion_Empty(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	var releases []githubapi.RepositoryRelease
	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/"+RunnerGhOrg+"/"+RunnerGhRepo+"/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	_, err := GetLatestCloudVersion()
	assert.NotNil(t, err, "error should occurred")

	_, err = GetLatestDockerVersion()
	assert.NotNil(t, err, "error should occurred")
}