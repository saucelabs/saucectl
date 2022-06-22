package framework

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/saucelabs/saucectl/internal/msg"
)

// Framework represents a test framework (e.g. cypress).
type Framework struct {
	Name    string
	Version string
}

// MetadataService represents an interface for retrieving framework metadata.
type MetadataService interface {
	Search(ctx context.Context, opts SearchOptions) (Metadata, error)

	Frameworks(ctx context.Context) ([]Framework, error)
	Versions(ctx context.Context, frameworkName string) ([]Metadata, error)
}

// SearchOptions represents read query options for MetadataService.Search().
type SearchOptions struct {
	Name             string
	FrameworkVersion string
}

// Metadata represents test runner metadata.
type Metadata struct {
	FrameworkName      string
	FrameworkVersion   string
	Deprecated         bool
	DockerImage        string
	GitRelease         string
	Platforms          []Platform
	CloudRunnerVersion string
}

// Platform represent a supported platform.
type Platform struct {
	PlatformName string
	BrowserNames []string
}

// GitReleaseSegments segments GitRelease into separate parts of org, repo and tag.
// Returns an error if GitRelease is malformed.
// The expected GitRelease format is "org/repo:tag".
func GitReleaseSegments(m *Metadata) (org, repo, tag string, err error) {
	// :punct: is a bit more generous than what would actually appear, but was chosen for the sake for readability.
	r := regexp.MustCompile(`(?P<org>[[:punct:][:word:]]+)/(?P<repo>[[:punct:][:word:]]+):(?P<tag>[[:punct:][:word:]]+)`)
	matches := r.FindStringSubmatch(m.GitRelease)

	// We expect a full match, plus 3 subgroups (org, repo tag). Thus a total of 4.
	if len(matches) != 4 {
		return "", "", "", errors.New(msg.InvalidGitRelease)
	}

	orgIndex := r.SubexpIndex("org")
	repoIndex := r.SubexpIndex("repo")
	tagIndex := r.SubexpIndex("tag")

	return matches[orgIndex], matches[repoIndex], matches[tagIndex], nil
}

// HasPlatform returns true if the provided Metadata has a matching platform.
func HasPlatform(m Metadata, platform string) bool {
	for _, p := range m.Platforms {
		if strings.ToLower(platform) == strings.ToLower(p.PlatformName) {
			return true
		}
	}

	return false
}

// PlatformNames extracts platform names from the given platforms and returns them.
func PlatformNames(platforms []Platform) []string {
	var pp []string
	for _, platform := range platforms {
		pp = append(pp, platform.PlatformName)
	}

	return pp
}
