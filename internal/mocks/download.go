package mocks

// FakeDownloader defines a fake Downloader
type FakeDownloader struct {
	DownloadArtifactsFn func(jobID string, passed bool)
}

// DownloadArtifacts defines a fake function for FakeDownloader
func (f *FakeDownloader) DownloadArtifacts(jobID string, passed bool) {
	f.DownloadArtifactsFn(jobID, passed)
}
