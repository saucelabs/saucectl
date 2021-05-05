package artifact

import (
	"context"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/ryanuber/go-glob"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
)

// Download defines artifacts downloading options
type Download struct {
	JobReader job.Reader
	Config    config.ArtifactDownload
}

// Download downloads artifacts according to config
func (d *Download) Download(jobID string) {
	targetDir := filepath.Join(d.Config.Directory, jobID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Error().Msgf("Unable to create %s to fetch artifacts (%v)", targetDir, err)
		return
	}

	files, err := d.JobReader.GetJobAssetFileNames(context.Background(), jobID)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return
	}
	for _, f := range files {
		for _, pattern := range d.Config.Match {
			if glob.Glob(pattern, f) {
				if err := d.downloadArtifact(targetDir, jobID, f); err != nil {
					log.Error().Err(err).Msgf("Failed to download file: %s", f)
				}
				break
			}
		}
	}
}

func (d *Download) downloadArtifact(targetDir, jobID, fileName string) error {
	content, err := d.JobReader.GetJobAssetFileContent(context.Background(), jobID, fileName)
	if err != nil {
		return err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return os.WriteFile(targetFile, content, 0644)
}

// ShouldDownload returns true if it should download artifacts, otherwise false
func ShouldDownload(jobID string, passed bool, cfg config.ArtifactDownload) bool {
	if jobID == "" {
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
