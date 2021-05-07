package mocks

// FakeArifactDownloader defines a fake Downloader
type FakeArifactDownloader struct {
	DownloadArtifactFn func(jobID string)
}

// DownloadArtifact defines a fake function for FakeDownloader
func (f *FakeArifactDownloader) DownloadArtifact(jobID string) {
	f.DownloadArtifactFn(jobID)
}
