package appstore

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/saucelabs/saucectl/internal/fileuploader"
)

// AppStore implements functions for AppStore interface
type AppStore struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

// New returns an implementation for AppStore
func New(url, username, accessKey string, timeout int) fileuploader.FileUploader {
	return &AppStore{
		HTTPClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// Upload uploads file to remote storage
func (s *AppStore) Upload(fileName, formType string) error {
	body, contentType, err := readFile(fileName, formType)
	if err != nil {
		return err
	}

	request, err := createRequest(s.URL, s.Username, s.AccessKey, body, contentType)
	if err != nil {
		return err
	}

	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	return err
}

func readFile(fileName, formType string) (*bytes.Buffer, string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()
	part, err := writer.CreateFormFile(formType, filepath.Base(file.Name()))
	if err != nil {
		return nil, "", err
	}
	io.Copy(part, file)

	return body, writer.FormDataContentType(), nil
}

func createRequest(url, username, accesskey string, body *bytes.Buffer, contentType string) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", contentType)
	request.SetBasicAuth(username, accesskey)

	return request, nil
}
