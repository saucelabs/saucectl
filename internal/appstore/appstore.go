package appstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
	"github.com/saucelabs/saucectl/internal/storage"
)

// UploadResponse represents the response as is returned by the app store.
type UploadResponse struct {
	Item Item `json:"item"`
}

// ListResponse represents the response as is returned by the app store.
type ListResponse struct {
	Items      []Item `json:"items"`
	Links      Links  `json:"links"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	TotalItems int    `json:"total_items"`
}

// Links represents the pagination information returned by the app store.
type Links struct {
	Self string `json:"self"`
	Prev string `json:"prev"`
	Next string `json:"next"`
}

// Item represents the metadata about the uploaded file.
type Item struct {
	ID   string `json:"id"`
	ETag string `json:"etag"`
}

// AppStore implements a remote file storage for storage.ProjectUploader.
// See https://wiki.saucelabs.com/display/DOCS/Application+Storage for more details.
type AppStore struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

// New returns an implementation for AppStore
func New(url, username, accessKey string, timeout time.Duration) *AppStore {
	return &AppStore{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// Upload uploads file to remote storage
func (s *AppStore) Upload(name string) (storage.ArtifactMeta, error) {
	body, contentType, err := readFile(name)
	if err != nil {
		return storage.ArtifactMeta{}, err
	}

	request, err := createRequest(fmt.Sprintf("%s/v1/storage/upload", s.URL), s.Username, s.AccessKey, body, contentType)
	if err != nil {
		return storage.ArtifactMeta{}, err
	}

	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		if err.(*url.Error).Timeout() {
			msg.LogUploadTimeoutSuggestion()
			return storage.ArtifactMeta{}, errors.New("failed to upload project")
		}
		return storage.ArtifactMeta{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return storage.ArtifactMeta{}, err
		}
		log.Error().Msgf("Failed to upload project. Invalid response %d, body: %v", resp.StatusCode, string(b))
		return storage.ArtifactMeta{}, errors.New("failed to upload project")
	}

	var ur UploadResponse

	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return storage.ArtifactMeta{}, err
	}

	return storage.ArtifactMeta{ID: ur.Item.ID}, err
}

func readFile(fileName string) (*bytes.Buffer, string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()
	part, err := writer.CreateFormFile("payload", filepath.Base(file.Name()))
	if err != nil {
		return nil, "", err
	}
	io.Copy(part, file)

	return body, writer.FormDataContentType(), nil
}

func createRequest(url, username, accesskey string, body *bytes.Buffer, contentType string) (*http.Request, error) {
	req, err := requesth.New(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(username, accesskey)

	return req, nil
}

// Locate looks for a file having the same hash.
func (s *AppStore) Locate(hash string) (storage.ArtifactMeta, error) {
	if hash == "" {
		return storage.ArtifactMeta{}, nil
	}
	queryString := ""
	for {
		request, err := createLocateRequest(fmt.Sprintf("%s/v1/storage/list", s.URL), s.Username, s.AccessKey, queryString)
		if err != nil {
			return storage.ArtifactMeta{}, err
		}

		lr, err := s.executeLocateRequest(request)
		if err != nil {
			return storage.ArtifactMeta{}, err
		}

		for _, item := range lr.Items {
			if item.ETag == hash {
				return storage.ArtifactMeta{ID: item.ID}, nil
			}
		}

		queryString = lr.Links.Next
		if queryString == "" {
			return storage.ArtifactMeta{}, nil
		}
	}
}

func createLocateRequest(url, username, accesskey string, queryString string) (*http.Request, error) {
	req, err := requesth.New(http.MethodGet, fmt.Sprintf("%s%s", url, queryString), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, accesskey)
	return req, nil
}

func (s *AppStore) executeLocateRequest(request *http.Request) (ListResponse, error) {
	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		return ListResponse{}, err

	}
	defer resp.Body.Close()

	var lr ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return ListResponse{}, err
	}
	return lr, nil
}
