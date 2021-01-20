package cypress

import (
	"fmt"
	"strings"

	"github.com/saucelabs/saucectl/internal/github"
)

// IsCypressVersionAvailable checks if the requested version is available on cloud or docker.
func IsCypressVersionAvailable(version string) (DockerAvailability, CloudAvailability bool, err error) {
	releases, err := github.GetReleases(RunnerGhOrg, RunnerGhRepo)
	if err != nil {
		return false, false, err
	}
	for _, release := range releases {
		if StandardizeVersionFormat(release.VersionNumber) == version {
			return true, release.CloudAvailability, nil
		}
	}
	return false, false, nil
}

// StandardizeVersionFormat remove the leading v in version to ensure reliable comparisons.
func StandardizeVersionFormat(version string) string {
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
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
	return StandardizeVersionFormat(releases[0].VersionNumber), nil
}

// GetLatestCloudVersion get the latest version available in sauce cloud.
func GetLatestCloudVersion() (string, error) {
	releases, err := github.GetReleases(RunnerGhOrg, RunnerGhRepo)
	if err != nil {
		return "", err
	}
	for _, release := range releases {
		if release.CloudAvailability {
			return StandardizeVersionFormat(release.VersionNumber), nil
		}
	}
	return "", fmt.Errorf("no cypress cloud version found")
}