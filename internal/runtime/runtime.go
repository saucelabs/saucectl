package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/msg"
	"golang.org/x/mod/semver"
)

const NodeRuntime = "nodejs"

// runtimeDisplayNames maps runtime identifiers to their human-readable display names.
var runtimeDisplayNames = map[string]string{
	NodeRuntime: "Node.js",
}

// Runtime represents runtime details on the VM.
type Runtime struct {
	Name        string
	Alias       []string
	Version     string
	EOLDate     time.Time
	RemovalDate time.Time
	Default     bool
	Extra       map[string]string
}

// Find selects the appropriate runtime from a list of runtimes.
// It supports full SemVer matching, alias resolution, and fuzzy matching for major or major.minor versions.
// `version` is expected to always start with "v".
func Find(runtimes []Runtime, name, version string) (Runtime, error) {
	rts := filterByName(runtimes, name)
	if !semver.IsValid(version) {
		// If version is not a valid SemVer, check if it's using an alias (e.g., "lts" or code name).
		res, err := findRuntimeByAlias(rts, version)
		if err == nil {
			return res, nil
		}
		return Runtime{}, fmt.Errorf("invalid %s version %s", runtimeDisplayNames[name], version)
	}

	// If the version is a full SemVer (i.e., major.minor.patch), attempt exact match.
	if isFullVersion(version) {
		for _, r := range rts {
			if "v"+r.Version == version {
				return r, nil
			}
		}
		return Runtime{}, fmt.Errorf("no matching %s version found for %s", runtimeDisplayNames[name], version)
	}

	// Fuzzy matching:
	// Try to match on major.minor.
	if onlyHasMajorMinor(version) {
		majorMinor := semver.MajorMinor(version)
		for _, r := range rts {
			if strings.HasPrefix("v"+r.Version, majorMinor+".") {
				return r, nil
			}
		}
		return Runtime{}, fmt.Errorf("no matching %s version found for %s", runtimeDisplayNames[name], version)
	}

	// Try to match on major version only.
	if onlyHasMajor(version) {
		major := semver.Major(version)
		for _, r := range rts {
			if strings.HasPrefix("v"+r.Version, major+".") {
				return r, nil
			}
		}
	}

	return Runtime{}, fmt.Errorf("no matching %s version found for %s", runtimeDisplayNames[name], version)
}

// GetDefault returns the default version for the specified runtime.
func GetDefault(runtimes []Runtime, name string) (Runtime, error) {
	for _, r := range runtimes {
		if r.Name == name && r.Default {
			return r, nil
		}
	}

	return Runtime{}, fmt.Errorf("no default version found for %s", runtimeDisplayNames[name])
}

func findRuntimeByAlias(runtimes []Runtime, alias string) (Runtime, error) {
	als := strings.ToLower(alias)
	for _, r := range runtimes {
		for _, a := range r.Alias {
			if als == a {
				return r, nil
			}
		}
	}

	return Runtime{}, fmt.Errorf("alias %q not found", alias)
}

func filterByName(runtimes []Runtime, name string) []Runtime {
	var rts []Runtime
	for _, r := range runtimes {
		if r.Name == name {
			rts = append(rts, r)
		}
	}
	return rts
}

func onlyHasMajor(version string) bool {
	return len(strings.Split(version, ".")) == 1
}

func onlyHasMajorMinor(version string) bool {
	return len(strings.Split(version, ".")) == 2
}

// isFullVersion checks if version follows the full semver format of `{major}.{minor}.{patch}`.
func isFullVersion(version string) bool {
	return len(strings.Split(version, ".")) == 3
}

func (r *Runtime) Validate(runtimes []Runtime) error {
	now := time.Now()
	if now.After(r.RemovalDate) {
		fmt.Print(msg.RemovalNotice(r.Name, r.Version, getAvailableRuntimes(runtimes, r.Name)))
		return fmt.Errorf("unsupported runtime %s(%s)", runtimeDisplayNames[r.Name], r.Version)
	}

	if now.After(r.EOLDate) {
		fmt.Print(msg.EOLNotice(r.Name, r.Version, r.RemovalDate, getAvailableRuntimes(runtimes, r.Name)))
	}
	return nil
}

func getAvailableRuntimes(runtimes []Runtime, name string) []string {
	now := time.Now()
	var versions []string
	for _, r := range runtimes {
		if r.Name == name && now.Before(r.EOLDate) {
			versions = append(versions, r.Version)
		}
	}
	return versions
}
