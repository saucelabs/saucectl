package storage

import "io"

// ProjectUploader is the interface for uploading bundled project files, later to be used in the Sauce Cloud.
type ProjectUploader interface {
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
