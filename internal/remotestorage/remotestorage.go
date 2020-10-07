package remotestorage

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// RemoteStorage defines functions to interact with application storage service
type RemoteStorage interface {
	Upload(url, fileName, formType string) ([]byte, error)
}

// Storage implements functions for RemoteStorage interface
type Storage struct {
	HTTPClient *http.Client
}

// NewRemoteStorage returns an implementation for RemoteStorage
func NewRemoteStorage() RemoteStorage {
	return &Storage{
		HTTPClient: &http.Client{},
	}
}

// Upload uploads file to remote storage
func (s *Storage) Upload(url, fileName, formType string) ([]byte, error) {
	// 1. prepare file
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(formType, filepath.Base(file.Name()))
	if err != nil {
		return nil, err
	}
	io.Copy(part, file)
	writer.Close()

	// 2. create request
	request, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.SetBasicAuth(os.Getenv("SAUCE_USERNAME"), os.Getenv("SAUCE_ACCESS_KEY"))

	// 3. send request
	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 4. return resp body
	return ioutil.ReadAll(resp.Body)
}
