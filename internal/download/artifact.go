package download

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
)

// ShouldDownloadArtifact returns true if it should download artifacts, otherwise false
func ShouldDownloadArtifact(jobID string, passed, timedOut bool, async bool, cfg config.ArtifactDownload) bool {
	if jobID == "" || timedOut || async {
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

// Cleanup removes previous downloaded artifacts
func Cleanup(directory string) {
	err := os.RemoveAll(directory)
	if err != nil {
		log.Err(err).Msg("Unable to cleanup previous artifacts")
	}
}
