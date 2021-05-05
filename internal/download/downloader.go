package download

// Downloader defines download functions
type Downloader interface {
	DownloadArtifacts(jobID string, passed bool)
}
