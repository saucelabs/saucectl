package github

import (
	"context"
	githubapi "github.com/google/go-github/v32/github"
	"time"
)

// Release represents a released version of a Github project
type Release struct {
	VersionNumber     string
	CloudAvailability bool
}

// GetReleases returns the list of releases for a Github project
func GetReleases(org, repo string) ([]Release, error) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	client := githubapi.NewClient(nil)
	opts := githubapi.ListOptions{

	}
	releases, _, err := client.Repositories.ListReleases(ctx, org, repo, &opts)
	if err != nil {
		return nil, err
	}
	var r []Release
	for _, release := range releases {
		r = append(r, Release{
			VersionNumber:     *release.Name,
			CloudAvailability: !*release.Prerelease,
		})
	}
	return r, nil
}
