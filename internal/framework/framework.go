package framework

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/mod/semver"
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

// Runtime represents runtime details on the VM.
type Runtime struct {
	RuntimeName    string
	RuntimeAlias   []string
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

// SupportGlobalNode checks if the current runner supports the global node.
func (m *Metadata) SupportGlobalNode() bool {
	if len(m.Runtimes) == 0 {
		return false
	}
	for _, r := range m.Runtimes {
		if r == NodeRuntime {
			return true
		}
	}

	return false
}

func findRuntimeByAlias(runtimes []Runtime, alias string) Runtime {
	for _, r := range runtimes {
		for _, a := range r.RuntimeAlias {
			if alias == a {
				return r
			}
		}
	}

	return Runtime{}
}

func filterNodeRuntimes(runtimes []Runtime) []Runtime {
	var nodeRuntimes []Runtime
	for _, r := range runtimes {
		if r.RuntimeName == NodeRuntime {
			nodeRuntimes = append(nodeRuntimes, r)
		}
	}
	return nodeRuntimes
}

func onlyHasMajor(version string) bool {
	return len(strings.Split(version, ".")) == 1
}

func onlyHasMajorMinor(version string) bool {
	return len(strings.Split(version, ".")) == 2
}

// isFullVersion checks if it contains major, minor and patch.
func isFullVersion(version string) bool {
	return len(strings.Split(version, ".")) == 3
}

// SelectNode selects the appropriate Node.js runtime from a list of runtimes.
// It supports full SemVer matching, alias resolution, and fuzzy matching for major or major.minor versions.
// `version` is expected to always start with "v".
func SelectNode(runtimes []Runtime, version string) (Runtime, error) {
	rts := filterNodeRuntimes(runtimes)
	if !semver.IsValid(version) {
		// If version is not a valid SemVer, check if it's using an alias (e.g., "lts" or code name).
		res := findRuntimeByAlias(rts, version)
		if res.RuntimeName != "" {
			return res, nil
		}
		return Runtime{}, fmt.Errorf("invalid node version %s", version)
	}

	// If the version is a full SemVer (i.e., major.minor.patch), attempt exact match.
	if isFullVersion(version) {
		for _, r := range rts {
			if "v"+r.RuntimeVersion == version {
				return r, nil
			}
		}
		return Runtime{}, fmt.Errorf("no matching node version found for %s", version)
	}

	// Fuzzy matching:
	// Try to match on major.minor.
	if onlyHasMajorMinor(version) {
		majorMinor := semver.MajorMinor(version)
		for _, r := range rts {
			if strings.HasPrefix("v"+r.RuntimeVersion, majorMinor+".") {
				return r, nil
			}
		}
		return Runtime{}, fmt.Errorf("no matching node version found for %s", version)
	}

	// If no match for major.minor, try to match on major version only.
	if onlyHasMajor(version) {
		major := semver.Major(version)
		for _, r := range rts {
			if strings.HasPrefix("v"+r.RuntimeVersion, major+".") {
				return r, nil
			}
		}
	}

	return Runtime{}, fmt.Errorf("no matching node version found for %s", version)
}

func ValidateRuntime(runtime Runtime) error {
	now := time.Now()
	if now.After(runtime.EOLDate) {
		return fmt.Errorf("node version %s has reached its EOL. Please upgrade to a newer version", runtime.RuntimeVersion)
	}
	return nil
}
