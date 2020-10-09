package appstore

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/saucelabs/saucectl/internal/storager"
)

// AppStore implements functions for AppStore interface
type AppStore struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

// New returns an implementation for AppStore
func New(url, username, accessKey string) storager.Storager {
	return &AppStore{
		HTTPClient: &http.Client{},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// Upload uploads file to remote storage
func (s *AppStore) Upload(fileName, formType string) error {
	body, writer, err := prepareFile(fileName, formType)
	if err != nil {
		return err
	}
	defer writer.Close()

	request, err := createRequest(s.URL, s.Username, s.AccessKey, body, writer)
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

func prepareFile(fileName, formType string) (*bytes.Buffer, *multipart.Writer, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(formType, filepath.Base(file.Name()))
	if err != nil {
		return nil, nil, err
	}
	io.Copy(part, file)

	return body, writer, nil
}

func createRequest(url, username, accesskey string, body *bytes.Buffer, writer *multipart.Writer) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.SetBasicAuth(username, accesskey)

	return request, nil
}
