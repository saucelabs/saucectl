package ci

import "os"

// IsAvailable detects whether this code is executed inside a CI environment.
func IsAvailable() bool {
	// Most CI providers have this.
	if os.Getenv("CI") != "" {
		return true
	}

	// Jenkins gets special treatment.
	return os.Getenv("BUILD_NUMBER") != ""
}
