package download

// ArtifactDownloader defines download functions
type ArtifactDownloader interface {
	DownloadArtifact(jobID string)
}
