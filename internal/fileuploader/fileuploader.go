package fileuploader

// FileUploader defines functions to interact with application storage service
type FileUploader interface {
	Upload(fileName, formType string) error
}
