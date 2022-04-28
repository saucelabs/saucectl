package mocks

// FakeArifactDownloader defines a fake Downloader
type FakeArifactDownloader struct {
	DownloadArtifactFn func(jobID, suiteName string) []string
}

// DownloadArtifact defines a fake function for FakeDownloader
func (f *FakeArifactDownloader) DownloadArtifact(jobID, suiteName string) []string {
	return f.DownloadArtifactFn(jobID, suiteName)
}
