package ci

import "os"

// IsAvailable detects whether this code is executed inside a CI environment.
func IsAvailable() bool {
	// Most CI providers have this.
	return os.Getenv("CI") != "" || os.Getenv("BUILD_NUMBER") != ""
}
