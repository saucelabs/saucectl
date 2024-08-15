package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

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

func (d *ArtifactDownloader) DownloadArtifact(jobID string, suiteName string, realDevice bool, attemptNumber int, timedOut bool, status string) []string {
	if jobID == "" || timedOut || !d.config.When.IsNow(status == job.StatePassed) || status == job.StateInProgress {
		return []string{}
	}

	destDir, err := config.GetSuiteArtifactFolder(suiteName, d.config)
	if err != nil {
		log.Error().Msgf("Unable to create artifacts folder (%v)", err)
		return []string{}
	}

	// FIXME: No magic numbers
	if attemptNumber != 0 {
		destDir = filepath.Join(destDir, strconv.Itoa(attemptNumber))
		err = os.Mkdir(destDir, 0755)
		if err != nil {
			log.Error().Msgf("Unable to create aritfacts folder (%v)", err)
			return []string{}
		}
	}

	files, err := d.reader.GetJobAssetFileNames(context.Background(), jobID, realDevice)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return []string{}
	}

	filepaths := fpath.MatchFiles(files, d.config.Match)
	var artifacts []string

	for _, f := range filepaths {
		targetFile, err := d.downloadArtifact(destDir, jobID, f, realDevice)
		if err != nil {
			log.Err(err).Msg("Unable to download artifacts")
			return artifacts
		}
		artifacts = append(artifacts, targetFile)
	}

	return artifacts
}

func (d *ArtifactDownloader) downloadArtifact(targetDir, jobID, fileName string, realDevice bool) (string, error) {
	log.Info().Str("fileName", fileName).Str("jobID", jobID).Msg("Downloading file")
	content, err := d.reader.GetJobAssetFileContent(context.Background(), jobID, fileName, realDevice)
	if err != nil {
		return "", err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return targetFile, os.WriteFile(targetFile, content, 0644)
}
