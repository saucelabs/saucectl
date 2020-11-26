package storage

// ProjectUploader is the interface for uploading bundled project files, later to be used in the Sauce Cloud.
type ProjectUploader interface {
	Upload(name string) (UploadResponse, error)
}

// UploadResponse represents the response that contains metadata about the uploaded file.
type UploadResponse struct {
	ID string
}
