package storage

// ProjectUploader is the interface for uploading bundled project files, later to be used in the Sauce Cloud.
type ProjectUploader interface {
	Upload(fileName, formType string) error
}
