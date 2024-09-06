package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
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
	Name    string
	Alias   []string
	Version string
	EOLDate time.Time
	Default bool
	Extra   map[string]string
}

// Find selects the appropriate runtime from a list of runtimes.
// It supports full SemVer matching, alias resolution, and fuzzy matching for major or major.minor versions.
// `version` is expected to always start with "v".
func Find(runtimes []Runtime, name, version string) (Runtime, error) {
	rts := filterByName(runtimes, name)
	if !semver.IsValid(version) {
		// If version is not a valid SemVer, check if it's using an alias (e.g., "lts" or code name).
		res, err := findRuntimeByAlias(rts, strings.ToLower(version))
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

func findRuntimeByAlias(runtimes []Runtime, alias string) (Runtime, error) {
	for _, r := range runtimes {
		for _, a := range r.Alias {
			if alias == a {
				return r, nil
			}
		}
	}

	return Runtime{}, fmt.Errorf("alias %q not found", alias)
}

func filterByName(runtimes []Runtime, name string) []Runtime {
	var rts []Runtime
	for _, r := range runtimes {
		if r.Name == NodeRuntime {
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

// isFullVersion checks if it contains major, minor and patch.
func isFullVersion(version string) bool {
	return len(strings.Split(version, ".")) == 3
}

func (r *Runtime) Validate() {
	now := time.Now()
	if now.After(r.EOLDate) {
		fmt.Printf(
			"%s%s%s",
			color.RedString(fmt.Sprintf("\n\n%s\n", msg.WarningLine)),
			color.RedString(fmt.Sprintf(
				"\nThe specified %s(%s) has reached its EOL. Please upgrade to a newer version.\n",
				runtimeDisplayNames[r.Name],
				r.Version,
			)),
			color.RedString(fmt.Sprintf("\n%s\n\n", msg.WarningLine)),
		)
	}
}
