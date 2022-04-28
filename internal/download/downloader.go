package download

// ArtifactDownloader defines download functions and returns downloaded file list
type ArtifactDownloader interface {
	DownloadArtifact(jobID, suiteName string) []string
}
