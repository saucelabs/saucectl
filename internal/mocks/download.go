package mocks

import "github.com/saucelabs/saucectl/internal/job"

// FakeArtifactDownloader defines a fake Downloader
type FakeArtifactDownloader struct {
	DownloadArtifactFn func(jobData job.Job, isLastAttempt bool) []string
}

// DownloadArtifact defines a fake function for FakeDownloader
func (f *FakeArtifactDownloader) DownloadArtifact(jobData job.Job, isLastAttempt bool) []string {
	return f.DownloadArtifactFn(jobData, isLastAttempt)
}
