package appstore

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// AppStore defines functions to interact with application storage service
type AppStore interface {
	Upload(fileName, formType string) (StorageResponse, error)
}

// StorageResponse defines response from remote storage service
type StorageResponse struct {
	Item struct {
		ID    string `json:"id"`
		Owner struct {
			ID    string `json:"id"`
			OrgID string `json:"org_id"`
		} `json:"owner"`
		Name            string      `json:"name"`
		UploadTimestamp int         `json:"upload_timestamp"`
		Etag            string      `json:"etag"`
		Kind            string      `json:"kind"`
		GroupID         int         `json:"group_id"`
		Metadata        interface{} `json:"metadata"`
		Access          struct {
			TeamIds []string `json:"team_ids"`
			OrgIds  []string `json:"org_ids"`
		} `json:"access"`
	} `json:"item"`
}

// Storage implements functions for AppStore interface
type Storage struct {
	HTTPClient *http.Client
	URL        string
}

// New returns an implementation for AppStore
func New(url string) AppStore {
	return &Storage{
		HTTPClient: &http.Client{},
		URL:        url,
	}
}

// Upload uploads file to remote storage
func (s *Storage) Upload(fileName, formType string) (data StorageResponse, err error) {
	body, writer, err := prepareFile(fileName, formType)
	if err != nil {
		return
	}
	defer writer.Close()

	request, err := createRequest(s.URL, body, writer)
	if err != nil {
		return
	}

	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	return parseResp(resp)
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

func createRequest(url string, body *bytes.Buffer, writer *multipart.Writer) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.SetBasicAuth(os.Getenv("SAUCE_USERNAME"), os.Getenv("SAUCE_ACCESS_KEY"))

	return request, nil
}

func parseResp(resp *http.Response) (StorageResponse, error) {
	data := StorageResponse{}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return data, err
	}

	return data, nil
}
