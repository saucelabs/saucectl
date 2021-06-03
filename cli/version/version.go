package version

// Default build-time variable.
// These values are overridden via ldflags
var (
	Version   = "v0.0.0+unknown"
	GitCommit = "unknown-commit-sha"
)

// Checker represents an interface for checking saucectl updates.
type Checker interface {
	HasUpdateAvailable() (string, error)
}
