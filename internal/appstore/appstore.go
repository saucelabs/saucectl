package appstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/multipartext"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

// errorResponse is a generic error response from the server.
type errorResponse struct {
	Code   int    `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// Links represents the pagination information returned by the app store.
type Links struct {
	Self string `json:"self"`
	Prev string `json:"prev"`
	Next string `json:"next"`
}

// Item represents the metadata about the uploaded file.
type Item struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Size            int    `json:"size"`
	UploadTimestamp int    `json:"upload_timestamp"`
}

// AppStore implements a remote file storage for storage.AppService.
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

// isMobileAppPackage determines if a file is a mobile app package.
func isMobileAppPackage(name string) bool {
	return strings.HasSuffix(name, ".ipa") || strings.HasSuffix(name, ".apk") || strings.HasSuffix(name, ".aab")
}

// Download downloads a file with the given id. It's the caller's responsibility to close the reader.
func (s *AppStore) Download(id string) (io.ReadCloser, int64, error) {
	req, err := requesth.New(http.MethodGet, fmt.Sprintf("%s/v1/storage/download/%s", s.URL, id), nil)
	if err != nil {
		return nil, 0, err
	}

	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	switch resp.StatusCode {
	case 200:
		return resp.Body, resp.ContentLength, nil
	case 400:
		return nil, 0, storage.ErrBadRequest // TODO consider parsing server response as well?
	case 401, 403:
		return nil, 0, storage.ErrAccessDenied
	case 404:
		return nil, 0, storage.ErrFileNotFound
	default:
		return nil, 0, newServerError(resp)
	}
}

// UploadStream uploads the contents of reader and stores them under the given filename.
func (s *AppStore) UploadStream(filename string, reader io.Reader) (storage.Item, error) {
	multipartReader, contentType, err := multipartext.NewMultipartReader(filename, reader)
	if err != nil {
		return storage.Item{}, err
	}

	req, err := requesth.New(http.MethodPost, fmt.Sprintf("%s/v1/storage/upload", s.URL), multipartReader)
	if err != nil {
		return storage.Item{}, err
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)

	switch resp.StatusCode {
	case 200, 201:
		var ur UploadResponse
		if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
			return storage.Item{}, err
		}

		return storage.Item{ID: ur.Item.ID}, err
	case 400:
		return storage.Item{}, storage.ErrBadRequest // TODO consider parsing server response as well?
	case 401, 403:
		return storage.Item{}, storage.ErrAccessDenied
	default:
		return storage.Item{}, newServerError(resp)
	}
}

// Upload uploads file to remote storage
func (s *AppStore) Upload(filename string) (storage.Item, error) {
	body, contentType, err := readFile(filename)
	if err != nil {
		return storage.Item{}, err
	}

	request, err := createRequest(fmt.Sprintf("%s/v1/storage/upload", s.URL), s.Username, s.AccessKey, body, contentType)
	if err != nil {
		return storage.Item{}, err
	}

	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		if err.(*url.Error).Timeout() {
			msg.LogUploadTimeout()
			if !isMobileAppPackage(filename) {
				msg.LogUploadTimeoutSuggestion()
			}
			return storage.Item{}, errors.New(msg.FailedToUpload)
		}
		return storage.Item{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return storage.Item{}, err
		}
		log.Error().Msgf("%s. Invalid response %d, body: %v", msg.FailedToUpload, resp.StatusCode, string(b))
		return storage.Item{}, errors.New(msg.FailedToUpload)
	}

	var ur UploadResponse

	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return storage.Item{}, err
	}

	return storage.Item{ID: ur.Item.ID}, err
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

	// FIXME This consumes quite a bit of memory (think of large mobile apps, node modules etc.).
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}

func createRequest(url, username, accesskey string, body io.Reader, contentType string) (*http.Request, error) {
	req, err := requesth.New(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(username, accesskey)

	return req, nil
}

// Find looks for a file having the same signature.
func (s *AppStore) Find(filename string) (storage.Item, error) {
	if filename == "" {
		return storage.Item{}, nil
	}

	hash, err := calculateBundleHash(filename)
	if err != nil {
		return storage.Item{}, err
	}
	log.Info().Msgf("Checksum: %s", hash)

	queryString := fmt.Sprintf("?sha256=%s", hash)
	request, err := createLocateRequest(fmt.Sprintf("%s/v1/storage/files", s.URL), s.Username, s.AccessKey, queryString)
	if err != nil {
		return storage.Item{}, err
	}

	lr, err := s.executeLocateRequest(request)
	if err != nil {
		return storage.Item{}, err
	}
	if lr.TotalItems == 0 {
		return storage.Item{}, nil
	}

	return storage.Item{ID: lr.Items[0].ID}, nil
}

func (s *AppStore) List(opts storage.ListOptions) (storage.List, error) {
	uri, _ := url.Parse(s.URL)
	uri.Path = "/v1/storage/files"

	query := uri.Query()
	query.Set("per_page", "100") // 100 is the max that app storage allows
	if opts.Q != "" {
		query.Set("q", opts.Q)
	}
	if opts.Name != "" {
		query.Set("name", opts.Name)
	}

	uri.RawQuery = query.Encode()

	req, err := requesth.New(http.MethodGet, uri.String(), nil)
	if err != nil {
		return storage.List{}, err
	}
	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return storage.List{}, err
	}
	defer resp.Body.Close()

	var listResp ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return storage.List{}, err
	}

	var items []storage.Item
	for _, v := range listResp.Items {
		items = append(items, storage.Item{
			ID:       v.ID,
			Name:     v.Name,
			Size:     v.Size,
			Uploaded: time.Unix(int64(v.UploadTimestamp), 0),
		})
	}

	return storage.List{
		Items:     items,
		Truncated: listResp.TotalItems > len(items),
	}, nil
}

func calculateBundleHash(filename string) (string, error) {
	fs, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer fs.Close()
	hsh := sha256.New()
	if _, err := io.Copy(hsh, fs); err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", hsh.Sum(nil))
	return hash, nil
}

func createLocateRequest(url, username, accesskey string, queryString string) (*http.Request, error) {
	req, err := requesth.New(http.MethodGet, fmt.Sprintf("%s%s&per_page=1", url, queryString), nil)
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

// newServerError inspects server error responses, trying to gather as much information as possible, especially if the body
// conforms to the errorResponse format, and returns a storage.ServerError.
func newServerError(resp *http.Response) *storage.ServerError {
	var errResp errorResponse
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	reader := bytes.NewReader(body)
	err := json.NewDecoder(reader).Decode(&errResp)
	if err != nil {
		return &storage.ServerError{
			Code:  resp.StatusCode,
			Title: resp.Status,
			Msg:   string(body),
		}
	}

	return &storage.ServerError{
		Code:  errResp.Code,
		Title: errResp.Title,
		Msg:   errResp.Detail,
	}
}
