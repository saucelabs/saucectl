package cypress

import (
	"fmt"
	"github.com/saucelabs/saucectl/internal/github"
)

// IsCypressVersionAvailable checks if the requested version is available on cloud or docker.
func IsCypressVersionAvailable(version string) (DockerAvailability, CloudAvailability bool, err error) {
	releases, err := github.GetReleases(RunnerGhOrg, RunnerGhRepo)
	if err != nil {
		return false, false, err
	}
	for _, release := range releases {
		if release.VersionNumber == version {
			return true, release.CloudAvailability, nil
		}
	}
	return false, false, nil
}

// GetLatestDockerVersion get the latest version available in docker mode.
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

// GetLatestCloudVersion get the latest version available in sauce cloud.
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