package storager

// Storager defines functions to interact with application storage service
type Storager interface {
	Upload(fileName, formType string) error
}
