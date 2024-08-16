package mocks

import "github.com/saucelabs/saucectl/internal/job"

// FakeArtifactDownloader defines a fake Downloader
type FakeArtifactDownloader struct {
	DownloadArtifactFn func(jobData job.Job, attempt int, retries int) []string
}

// DownloadArtifact defines a fake function for FakeDownloader
func (f *FakeArtifactDownloader) DownloadArtifact(jobData job.Job, attempt int, retries int) []string {
	return f.DownloadArtifactFn(jobData, attempt, retries)
}
