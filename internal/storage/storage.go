package storage

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// ListOptions represents parameters that modify the file list behavior.
type ListOptions struct {
	// Q is any search term (such as app name, file name, description, build number or version) by which you want to filter.
	Q string

	// Name The file name (case-insensitive) by which you want to filter.
	Name string
}

type List struct {
	Items     []Item `json:"items"`
	Truncated bool   `json:"truncated"`
}

// Item represents the file in storage.
type Item struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Size     int       `json:"size"`
	Uploaded time.Time `json:"uploaded"`
}

// ErrFileNotFound is returned when the requested file does not exist.
var ErrFileNotFound = errors.New("file not found")

// ErrAccessDenied is returned when the service denies access. Either due to insufficient rights or wrong credentials.
var ErrAccessDenied = errors.New("access denied")

// ServerError represents any server side error that isn't already covered by other types of errors in this package.
type ServerError struct {
	Code  int
	Title string
	Msg   string
}

func (s *ServerError) Error() string {
	return fmt.Sprintf("server error with status '%d'; title '%s'; msg '%s'", s.Code, s.Title, s.Msg)
}

// AppService is the interface for interacting with the Sauce application storage.
type AppService interface {
	// Deprecated: Use UploadStream.
	Upload(name string) (Item, error)
	// UploadStream uploads the contents of reader and stores them under the given filename.
	UploadStream(filename string, reader io.Reader) (Item, error)
	Download(id string) (io.ReadCloser, int64, error)
	Find(name string) (Item, error)
	List(opts ListOptions) (List, error)
}
