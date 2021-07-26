package download

import "github.com/saucelabs/saucectl/internal/config"

// ShouldDownloadArtifact returns true if it should download artifacts, otherwise false
func ShouldDownloadArtifact(jobID string, passed, timedOut bool, cfg config.ArtifactDownload) bool {
	if jobID == "" || timedOut {
		return false
	}
	if cfg.When == config.WhenAlways {
		return true
	}
	if cfg.When == config.WhenFail && !passed {
		return true
	}
	if cfg.When == config.WhenPass && passed {
		return true
	}

	return false
}
