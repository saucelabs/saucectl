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

func (d *ArtifactDownloader) DownloadArtifact(jobData job.Job, attemptNumber int, retries int, isRetriedJob bool) []string {
	if jobData.ID == "" ||
		jobData.TimedOut || !job.Done(jobData.Status) ||
		!d.config.When.IsNow(jobData.Passed) ||
		(isRetriedJob && !d.config.AllAttempts && attemptNumber < retries) {
		return []string{}
	}

	destDir, err := config.GetSuiteArtifactFolder(jobData.Name, d.config)
	if err != nil {
		log.Error().Msgf("Unable to create artifacts folder (%v)", err)
		return []string{}
	}

	files, err := d.reader.GetJobAssetFileNames(context.Background(), jobData.ID, jobData.IsRDC)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return []string{}
	}

	filepaths := fpath.MatchFiles(files, d.config.Match)
	var artifacts []string

	for _, f := range filepaths {
		targetFile, err := d.downloadArtifact(destDir, jobData.ID, f, jobData.IsRDC)
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
