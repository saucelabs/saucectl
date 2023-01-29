package mocks

// FakeArtifactDownloader defines a fake Downloader
type FakeArtifactDownloader struct {
	DownloadArtifactFn func(jobID, suiteName string) []string
}

// DownloadArtifact defines a fake function for FakeDownloader
func (f *FakeArtifactDownloader) DownloadArtifact(jobID, suiteName string, realDevice bool) []string {
	return f.DownloadArtifactFn(jobID, suiteName)
}
