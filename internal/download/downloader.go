package download

// Downloader defines download functions
type Downloader interface {
	Download(jobID string)
}
