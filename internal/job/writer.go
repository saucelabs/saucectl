package job

// Writer is the interface for modifying jobs.
type Writer interface {
	UploadAsset(jobID string, fileName string, content []byte) error
}