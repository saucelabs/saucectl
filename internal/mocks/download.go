package mocks

// FakeArifactDownloader defines a fake Downloader
type FakeArifactDownloader struct {
	DownloadArtifactFn func(jobID, suiteName string)
}

// DownloadArtifact defines a fake function for FakeDownloader
func (f *FakeArifactDownloader) DownloadArtifact(jobID, suiteName string) {
	f.DownloadArtifactFn(jobID, suiteName)
}
