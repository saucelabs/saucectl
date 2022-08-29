package storage

import "io"

// ProjectUploader is the interface for uploading bundled project files, later to be used in the Sauce Cloud.
type ProjectUploader interface {
	Upload(name string) (ArtifactMeta, error)
	Download(id string) (io.ReadCloser, error)
	Find(name string) (ArtifactMeta, error)
	List(opts ListOptions) (List, error)
}

// ArtifactMeta represents metadata of the uploaded file.
type ArtifactMeta struct {
	ID string
}
