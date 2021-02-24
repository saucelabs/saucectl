package version

// Default build-time variable.
// These values are overridden via ldflags
var (
	Version   = "v0.0.0+unknown"
	GitCommit = "unknown-commit-sha"
)
