package mocks

// FakeDownloader defines a fake Downloader
type FakeDownloader struct {
	DownloadFn func(jobID string)
}

// Download defines a fake function for FakeDownloader
func (f *FakeDownloader) Download(jobID string) {
	f.DownloadFn(jobID)
}
