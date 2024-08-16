package downloader

import (
	"context"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/job"
)

type ArtifactDownloader struct {
	reader job.Reader
	config config.ArtifactDownload
}

func NewArtifactDownloader(reader job.Reader, artifactConfig config.ArtifactDownload) ArtifactDownloader {
	return ArtifactDownloader{
		reader: reader,
		config: artifactConfig,
	}
}

func (d *ArtifactDownloader) DownloadArtifact(jobID string, suiteName string, realDevice bool) []string {
	targetDir, err := config.GetSuiteArtifactFolder(suiteName, d.config)
	if err != nil {
		log.Error().Msgf("Unable to create artifacts folder (%v)", err)
		return []string{}
	}
	files, err := d.reader.GetJobAssetFileNames(context.Background(), jobID, realDevice)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return []string{}
	}

	filepaths := fpath.MatchFiles(files, d.config.Match)
	var artifacts []string
	for _, f := range filepaths {
		targetFile, err := d.downloadArtifact(targetDir, jobID, f, realDevice)
		if err != nil {
			log.Err(err).Msg("Unable to download artifacts")
			return artifacts
		}
		artifacts = append(artifacts, targetFile)
	}

	return artifacts
}

func (d *ArtifactDownloader) downloadArtifact(targetDir, jobID, fileName string, realDevice bool) (string, error) {
	content, err := d.reader.GetJobAssetFileContent(context.Background(), jobID, fileName, realDevice)
	if err != nil {
		return "", err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return targetFile, os.WriteFile(targetFile, content, 0644)
}
