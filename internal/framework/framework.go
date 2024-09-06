package framework

import (
	"context"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/runtime"
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
	Runtimes(ctx context.Context) ([]runtime.Runtime, error)
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

// SupportsRuntime checks if the current runner supports the specified runtime.
func (m *Metadata) SupportsRuntime(runtimeName string) bool {
	if len(m.Runtimes) == 0 {
		return false
	}
	for _, r := range m.Runtimes {
		if r == runtimeName {
			return true
		}
	}

	return false
}
