package cypress

import (
	"fmt"
	"github.com/saucelabs/saucectl/internal/github"
)

// IsCypressVersionAvailable checks if the requested version is available on cloud or docker.
func IsCypressVersionAvailable(version string) (isCloudAvailable bool, err error) {
	releases, err := github.GetReleases(RunnerGhOrg, RunnerGhRepo)
	if err != nil {
		return false, err
	}
	for _, release := range releases {
		if release.VersionNumber == version {
			return release.CloudAvailability, nil
		}
	}
	return false, fmt.Errorf("cypress %s is not available", version)
}

func GetLatestDockerVersion() (string, error) {
	releases, err := github.GetReleases(RunnerGhOrg, RunnerGhRepo)
	if err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("no cypress version found")
	}
	return releases[0].VersionNumber, nil
}

func GetLatestCloudVersion() (string, error) {
	releases, err := github.GetReleases(RunnerGhOrg, RunnerGhRepo)
	if err != nil {
		return "", err
	}
	for _, release := range releases {
		if release.CloudAvailability {
			return releases[0].VersionNumber, nil
		}
	}
	return "", fmt.Errorf("no cypress cloud version found")
}