package framework

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Framework represents a test framework (e.g. cypress).
type Framework struct {
	Name    string
	Version string
}

// MetadataService represents an interface for retrieving framework metadata.
type MetadataService interface {
	Frameworks(ctx context.Context) ([]string, error)
	Versions(ctx context.Context, frameworkName string) ([]Metadata, error)
	Runtimes(ctx context.Context) ([]Runtime, error)
}

// Metadata represents test runner metadata.
type Metadata struct {
	FrameworkName      string
	FrameworkVersion   string
	EOLDate            time.Time
	RemovalDate        time.Time
	DockerImage        string
	GitRelease         string
	Platforms          []Platform
	CloudRunnerVersion string
	BrowserDefaults    map[string]string
	Runtimes           []string
}

type Runtime struct {
	RuntimeName    string
	RuntimeVersion string
	EOLDate        time.Time
	Default        bool
	Extra          map[string]string
}

const NodeRuntime = "nodejs"

func (m *Metadata) IsDeprecated() bool {
	return time.Now().After(m.EOLDate)
}

func (m *Metadata) IsFlaggedForRemoval() bool {
	return time.Now().After(m.RemovalDate)
}

// Platform represent a supported platform.
type Platform struct {
	PlatformName string
	BrowserNames []string
}

// HasPlatform returns true if the provided Metadata has a matching platform.
func HasPlatform(m Metadata, platform string) bool {
	for _, p := range m.Platforms {
		if strings.EqualFold(platform, p.PlatformName) {
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

func (m *Metadata) SupportGlobalNode() bool {
	return len(m.Runtimes) > 0
}

func SelectNodeVersion(runtimes []Runtime, version string) (string, error) {
	version = strings.ReplaceAll(version, "v", "")
	items := strings.Split(version, ".")
	var filtered []string
	for _, r := range runtimes {
		if r.RuntimeName == NodeRuntime {
			if len(items) < 3 && strings.HasPrefix(r.RuntimeVersion, version+".") {
				filtered = append(filtered, r.RuntimeVersion)
			} else if len(items) == 3 {
				filtered = append(filtered, r.RuntimeVersion)
			}
		}
	}

	if len(filtered) == 0 {
		return "", fmt.Errorf("no versions found for node version %q", version)
	}

	sort.Slice(filtered, func(a, b int) bool {
		return filtered[a] > filtered[b]
	})

	return filtered[0], nil
}

func ValidateNodeVersion(runtimes []Runtime, version string) error {
	version = strings.ReplaceAll(version, "v", "")
	items := strings.Split(version, ".")
	if len(items) < 3 {
		return nil
	}

	var found bool
	for _, r := range runtimes {
		if r.RuntimeName == NodeRuntime && r.RuntimeVersion == version {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no matching version found for node version %q", version)
	}

	return nil
}
