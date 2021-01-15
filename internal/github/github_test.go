package github

import (
	"fmt"
	githubapi "github.com/google/go-github/v32/github"
	"github.com/jarcoal/httpmock"
	"net/http"
	"testing"
)

func TestGetReleases(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	v0 := "5.6.2"
	v1 := "5.6.0"
	v2 := "5.5.0"
	preRelease := true
	release := false
	releases := []githubapi.RepositoryRelease{
		{ Name:  &v0, Prerelease: &preRelease },
		{ Name:  &v1, Prerelease: &release },
		{ Name:  &v2, Prerelease: &release },
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
		fmt.Printf("%s", err)
		t.Fail()
	}
	for _, rr := range r {
		fmt.Println(rr.VersionNumber)
	}
}