package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// ListOptions represents parameters that modify the file list behavior.
type ListOptions struct {
	// Q is any search term (such as app name, file name, description, build number or version) by which you want to filter.
	Q string

	// Name is the filename (case-insensitive) by which you want to filter.
	Name string

	// SHA256 is the checksum of the file by which you want to filter.
	SHA256 string

	// Limits the number of results returned.
	MaxResults int

	Tags []string
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
	Tags     []string
}

// ErrFileNotFound is returned when the requested file does not exist.
var ErrFileNotFound = errors.New("file not found")

// ErrAccessDenied is returned when the service denies access. Either due to insufficient rights or wrong credentials.
var ErrAccessDenied = errors.New("access denied")

// ErrTooManyRequest is returned when the request number is exceeding rate limit.
var ErrTooManyRequest = errors.New("too many requests, please try again later")

// ServerError represents any server side error that isn't already covered by other types of errors in this package.
type ServerError struct {
	Code  int
	Title string
	Msg   string
}

type FileInfo struct {
	Name        string
	Description string
	Tags        []string
}

func (s *ServerError) Error() string {
	return fmt.Sprintf("server error with status '%d'; title '%s'; msg '%s'", s.Code, s.Title, s.Msg)
}

// AppService is the interface for interacting with the Sauce application storage.
type AppService interface {
	// UploadStream uploads the contents of reader and stores them under the given file info.
	UploadStream(ctx context.Context, fileInfo FileInfo, reader io.Reader) (Item, error)
	Download(ctx context.Context, id string) (io.ReadCloser, int64, error)
	Delete(ctx context.Context, id string) error
	DownloadURL(ctx context.Context, url string) (io.ReadCloser, int64, error)
	List(ctx context.Context, opts ListOptions) (List, error)
}
