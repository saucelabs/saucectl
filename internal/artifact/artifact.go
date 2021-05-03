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

type ArtifactDownloader struct {
	JobReader job.Reader
}

func (d *ArtifactDownloader) DownloadArtifacts(artifactsCfg config.ArtifactDownload, jobID string, passed bool) {
	if !shouldDownloadArtifacts(artifactsCfg, jobID, passed) {
		return
	}
	targetDir := filepath.Join(artifactsCfg.Directory, jobID)
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
		for _, pattern := range artifactsCfg.Match {
			if glob.Glob(pattern, f) {
				if err := d.downloadArtifact(targetDir, jobID, f); err != nil {
					log.Error().Err(err).Msgf("Failed to download file: %s", f)
				}
				break
			}
		}
	}
}

func (d *ArtifactDownloader) downloadArtifact(targetDir, jobID, fileName string) error {
	content, err := d.JobReader.GetJobAssetFileContent(context.Background(), jobID, fileName)
	if err != nil {
		return err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return os.WriteFile(targetFile, content, 0644)
}

func shouldDownloadArtifacts(artifactsCfg config.ArtifactDownload, jobID string, passed bool) bool {
	if jobID == "" {
		return false
	}
	if artifactsCfg.When == config.WhenAlways {
		return true
	}
	if artifactsCfg.When == config.WhenFail && !passed {
		return true
	}
	if artifactsCfg.When == config.WhenPass && passed {
		return true
	}

	return false
}
