package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/multipartext"
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
	ID              string `json:"id"`
	Name            string `json:"name"`
	Size            int    `json:"size"`
	UploadTimestamp int64  `json:"upload_timestamp"`
}

// AppStore implements a remote file storage for storage.AppService.
// See https://wiki.saucelabs.com/display/DOCS/Application+Storage for more details.
type AppStore struct {
	HTTPClient *retryablehttp.Client
	URL        string
	Username   string
	AccessKey  string
}

// NewAppStore returns an implementation for AppStore
func NewAppStore(url, username, accessKey string, timeout time.Duration) *AppStore {
	return &AppStore{
		HTTPClient: NewRetryableClient(timeout),
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// Download downloads a file with the given id. It's the caller's responsibility to close the reader.
func (s *AppStore) Download(id string) (io.ReadCloser, int64, error) {
	req, err := retryablehttp.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/storage/download/%s", s.URL, id), nil)
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
	case 401, 403:
		return nil, 0, storage.ErrAccessDenied
	case 404:
		return nil, 0, storage.ErrFileNotFound
	case 429:
		return nil, 0, storage.ErrTooManyRequest
	default:
		return nil, 0, s.newServerError(resp)
	}
}

// DownloadURL downloads a file from the url. It's the caller's responsibility to close the reader.
func (s *AppStore) DownloadURL(url string) (io.ReadCloser, int64, error) {
	req, err := retryablehttp.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	switch resp.StatusCode {
	case 200:
		return resp.Body, resp.ContentLength, nil
	default:
		b, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("unexpected server response (%d): %s", resp.StatusCode, b)
	}
}

// UploadStream uploads the contents of reader and stores them under the given file info.
func (s *AppStore) UploadStream(fileInfo storage.FileInfo, reader io.Reader) (storage.Item, error) {
	multipartReader, contentType, err := multipartext.NewMultipartReader("payload", fileInfo, reader)
	if err != nil {
		return storage.Item{}, err
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/storage/upload", s.URL), multipartReader)
	if err != nil {
		return storage.Item{}, err
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return storage.Item{}, err
	}

	switch resp.StatusCode {
	case 200, 201:
		var ur UploadResponse
		if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
			return storage.Item{}, err
		}

		return storage.Item{
			ID:       ur.Item.ID,
			Name:     ur.Item.Name,
			Uploaded: time.Unix(ur.Item.UploadTimestamp, 0),
			Size:     ur.Item.Size,
		}, err
	case 401, 403:
		return storage.Item{}, storage.ErrAccessDenied
	case 429:
		return storage.Item{}, storage.ErrTooManyRequest
	default:
		return storage.Item{}, s.newServerError(resp)
	}
}

// List returns a list of items stored in the Sauce app storage that match the search criteria specified by opts.
func (s *AppStore) List(opts storage.ListOptions) (storage.List, error) {
	uri, _ := url.Parse(s.URL)
	uri.Path = "/v1/storage/files"

	// Default MaxResults if not set or out of range.
	if opts.MaxResults < 1 || opts.MaxResults > 100 {
		opts.MaxResults = 100
	}

	query := uri.Query()
	query.Set("per_page", strconv.Itoa(opts.MaxResults))
	if opts.MaxResults == 1 {
		query.Set("paginate", "no")
	}
	if opts.Q != "" {
		query.Set("q", opts.Q)
	}
	if opts.Name != "" {
		query.Set("name", opts.Name)
	}
	if opts.SHA256 != "" {
		query.Set("sha256", opts.SHA256)
	}
	for _, t := range opts.Tags {
		query.Add("tags", t)
	}

	uri.RawQuery = query.Encode()

	req, err := retryablehttp.NewRequest(http.MethodGet, uri.String(), nil)
	if err != nil {
		return storage.List{}, err
	}
	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return storage.List{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
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
				Uploaded: time.Unix(v.UploadTimestamp, 0),
			})
		}

		return storage.List{
			Items:     items,
			Truncated: listResp.TotalItems > len(items),
		}, nil
	case 401, 403:
		return storage.List{}, storage.ErrAccessDenied
	case 429:
		return storage.List{}, storage.ErrTooManyRequest
	default:
		return storage.List{}, s.newServerError(resp)
	}
}

func (s *AppStore) Delete(id string) error {
	if id == "" {
		return fmt.Errorf("no id specified")
	}

	req, err := retryablehttp.NewRequest(http.MethodDelete, fmt.Sprintf("%s/v1/storage/files/%s", s.URL, id), nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case 200:
		return nil
	case 401, 403:
		return storage.ErrAccessDenied
	case 404:
		return storage.ErrFileNotFound
	case 429:
		return storage.ErrTooManyRequest
	default:
		return s.newServerError(resp)
	}
}

// newServerError inspects server error responses, trying to gather as much information as possible, especially if the body
// conforms to the errorResponse format, and returns a storage.ServerError.
func (s *AppStore) newServerError(resp *http.Response) *storage.ServerError {
	var errResp struct {
		Code   int    `json:"code"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
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
