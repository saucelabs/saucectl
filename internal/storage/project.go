package storage

import "io"

// AppService is the interface for interacting with the Sauce application storage.
type AppService interface {
	Upload(name string) (ArtifactMeta, error)
	// UploadStream uploads the contents of reader and stores them under the given filename.
	UploadStream(filename string, reader io.Reader) (ArtifactMeta, error)
	Download(id string) (io.ReadCloser, int64, error)
	Find(name string) (ArtifactMeta, error)
	List(opts ListOptions) (List, error)
}

// ArtifactMeta represents metadata of the uploaded file.
type ArtifactMeta struct {
	ID string
}
