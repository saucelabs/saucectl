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

// Downloader defines artifacts downloader
type Downloader struct {
	JobReader job.Reader
	Config    config.ArtifactDownload
}

// Download downloads artifacts according to config
func (d *Downloader) Download(jobID string, passed bool) {
	if !shouldDownload(d.Config, jobID, passed) {
		return
	}
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

func (d *Downloader) downloadArtifact(targetDir, jobID, fileName string) error {
	content, err := d.JobReader.GetJobAssetFileContent(context.Background(), jobID, fileName)
	if err != nil {
		return err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return os.WriteFile(targetFile, content, 0644)
}

func shouldDownload(Config config.ArtifactDownload, jobID string, passed bool) bool {
	if jobID == "" {
		return false
	}
	if Config.When == config.WhenAlways {
		return true
	}
	if Config.When == config.WhenFail && !passed {
		return true
	}
	if Config.When == config.WhenPass && passed {
		return true
	}

	return false
}
