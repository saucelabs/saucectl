package storage

// ProjectUploader is the interface for uploading bundled project files, later to be used in the Sauce Cloud.
type ProjectUploader interface {
	Upload(name string) (ArtifactMeta, error)
	Find(name string) (ArtifactMeta, error)
}

// ArtifactMeta represents metadata of the uploaded file.
type ArtifactMeta struct {
	ID string
}