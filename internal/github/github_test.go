package github

import (
	githubapi "github.com/google/go-github/v32/github"
	"github.com/jarcoal/httpmock"
	"gotest.tools/assert"
	"net/http"
	"testing"
)

func TestGetReleases(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	v1 := "5.6.0"
	v2 := "5.5.0"
	trueValue := true
	falseValue := false
	releases := []githubapi.RepositoryRelease{
		{ Name:  &v0, Prerelease: &trueValue },
		{ Name:  &v1, Prerelease: &falseValue },
		{ Name:  &v2, Prerelease: &falseValue },
	}
	expectedReleases := []Release{
		{ v0,  false},
		{ v1,  true},
		{ v2,  true},
	}

	httpmock.RegisterResponder(http.MethodGet, "https://api.github.com/repos/fake-org/fake-repo/releases",
		func(req *http.Request) (*http.Response, error) {
			resp, err := httpmock.NewJsonResponse(200, releases)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), nil
			}
			return resp, nil
		},
	)

	r, err := GetReleases("fake-org", "fake-repo")
	if err != nil {
		t.Fail()
	}
	assert.DeepEqual(t, r, expectedReleases)
}