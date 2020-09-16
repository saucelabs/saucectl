package ci

import "os"

// IsAvailable detects whether this code is executed inside a CI environment
func IsAvailable() bool {
	// Most CI providers have this.
	isCi := os.Getenv("CI") != "" || os.Getenv("BUILD_NUMBER") != ""
	skip := os.Getenv("SKIP_CI") != ""
	return isCi && !skip
}
