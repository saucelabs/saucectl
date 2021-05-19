package resto

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/ryanuber/go-glob"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/requesth"

	"github.com/saucelabs/saucectl/internal/job"
)

var (
	// ErrServerError is returned when the server was not able to correctly handle our request (status code >= 500).
	ErrServerError = errors.New("internal server error")
	// ErrJobNotFound is returned when the requested job was not found.
	ErrJobNotFound = errors.New("job was not found")
	// ErrTunnelNotFound is returned when the requested tunnel was not found.
	ErrTunnelNotFound = errors.New("tunnel not found")
)

// Client http client.
type Client struct {
	HTTPClient     *http.Client
	URL            string
	Username       string
	AccessKey      string
	ArtifactConfig config.ArtifactDownload
}

// concurrencyResponse is the response body as is returned by resto's rest/v1.2/users/{username}/concurrency endpoint.
type concurrencyResponse struct {
	Concurrency struct {
		Organization struct {
			Allowed struct {
				VMS int `json:"vms"`
				RDS int `json:"rds"`
			}
		}
	}
}

// availableTunnelsResponse is the response body as is returned by resto's rest/v1.1/users/{username}/available_tunnels endpoint.
type availableTunnelsResponse map[string][]tunnel

type tunnel struct {
	ID       string `json:"id"`
	Status   string `json:"status"` // 'new', 'booting', 'deploying', 'halting', 'running', 'terminated'
	TunnelID string `json:"tunnel_identifier"`
}

// New creates a new client.
func New(url, username, accessKey string, timeout time.Duration) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// ReadJob returns the job details.
func (c *Client) ReadJob(ctx context.Context, id string) (job.Job, error) {
	request, err := createRequest(ctx, c.URL, c.Username, c.AccessKey, id)
	if err != nil {
		return job.Job{}, err
	}

	return doRequest(c.HTTPClient, request)
}

// PollJob polls job details at an interval, until the job has ended, whether successfully or due to an error.
func (c *Client) PollJob(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
	request, err := createRequest(ctx, c.URL, c.Username, c.AccessKey, id)
	if err != nil {
		return job.Job{}, err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		j, err := doRequest(c.HTTPClient, request)
		if err != nil {
			return job.Job{}, err
		}

		if job.Done(j.Status) {
			return j, nil
		}
	}

	return job.Job{}, nil
}

// GetJobAssetFileNames return the job assets list.
func (c *Client) GetJobAssetFileNames(ctx context.Context, jobID string) ([]string, error) {
	request, err := createListAssetsRequest(ctx, c.URL, c.Username, c.AccessKey, jobID)
	if err != nil {
		return nil, err
	}
	return doListAssetsRequest(c.HTTPClient, request)
}

// GetJobAssetFileContent returns the job asset file content.
func (c *Client) GetJobAssetFileContent(ctx context.Context, jobID, fileName string) ([]byte, error) {
	request, err := createAssetRequest(ctx, c.URL, c.Username, c.AccessKey, jobID, fileName)
	if err != nil {
		return nil, err
	}

	return doAssetRequest(c.HTTPClient, request)
}

// ReadAllowedCCY returns the allowed (max) concurrency for the current account.
func (c *Client) ReadAllowedCCY(ctx context.Context) (int, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1.2/users/%s/concurrency", c.URL, c.Username), nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var cr concurrencyResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return 0, err
	}

	return cr.Concurrency.Organization.Allowed.VMS, nil
}

// IsTunnelRunning checks whether tunnelID is running. If not, it will wait for the tunnel to become available or
// timeout. Whichever comes first.
func (c *Client) IsTunnelRunning(ctx context.Context, id string, wait time.Duration) error {
	deathclock := time.Now().Add(wait)
	var err error
	for time.Now().Before(deathclock) {
		if err = c.isTunnelRunning(ctx, id); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return err
}

func (c *Client) isTunnelRunning(ctx context.Context, id string) error {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1.1/%s/available_tunnels", c.URL, c.Username), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	q := req.URL.Query()
	q.Add("full", "true")
	req.URL.RawQuery = q.Encode()

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err := fmt.Errorf("tunnel request failed; unexpected response code:'%d', msg:'%v'", res.StatusCode, string(body))
		return err
	}

	var resp availableTunnelsResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return err
	}

	for _, tt := range resp {
		for _, t := range tt {
			// User could be using tunnel name (aka tunnel_identifier) or the tunnel ID. Make sure we check both.
			if t.TunnelID != id && t.ID != id {
				continue
			}
			if t.Status == "running" {
				return nil
			}
		}
	}
	return ErrTunnelNotFound
}

// StopJob stops the job on the Sauce Cloud.
func (c *Client) StopJob(ctx context.Context, id string) (job.Job, error) {
	request, err := createStopRequest(ctx, c.URL, c.Username, c.AccessKey, id)
	if err != nil {
		return job.Job{}, err
	}
	j, err := doRequest(c.HTTPClient, request)
	if err != nil {
		return job.Job{}, err
	}
	return j, nil
}

// UploadAsset uploads an asset to the specified jobID.
func (c *Client) UploadAsset(jobID string, fileName string, contentType string, content []byte) error {
	request, err := createUploadAssetRequest(context.Background(), c.URL, c.Username, c.AccessKey, jobID, fileName, contentType, content)
	if err != nil {
		return err
	}
	err = doRequestAsset(c.HTTPClient, request)
	return err
}

func doListAssetsRequest(httpClient *http.Client, request *http.Request) ([]string, error) {
	resp, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, ErrServerError
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrJobNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("job assets list request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return nil, err
	}

	var filesMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&filesMap); err != nil {
		return []string{}, err
	}

	var filesList []string
	for k, v := range filesMap {
		if v != nil && !isSpecialFile(k) && reflect.TypeOf(v).Name() == "string" {
			filesList = append(filesList, v.(string))
		}
	}
	return filesList, nil
}

// isSpecialFile tells if a file is a specific case or not.
func isSpecialFile(fileName string) bool {
	if fileName == "video" || fileName == "screenshots" {
		return true
	}
	return false
}

func doAssetRequest(httpClient *http.Client, request *http.Request) ([]byte, error) {
	resp, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, ErrServerError
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrJobNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("job status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

func doRequest(httpClient *http.Client, request *http.Request) (job.Job, error) {
	resp, err := httpClient.Do(request)
	if err != nil {
		return job.Job{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return job.Job{}, ErrServerError
	}

	if resp.StatusCode == http.StatusNotFound {
		return job.Job{}, ErrJobNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("job status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return job.Job{}, err
	}

	jobDetails := job.Job{}
	if err := json.NewDecoder(resp.Body).Decode(&jobDetails); err != nil {
		return job.Job{}, err
	}

	return jobDetails, nil
}

func createRequest(ctx context.Context, url, username, accessKey, jobID string) (*http.Request, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1.1/%s/jobs/%s", url, username, jobID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, accessKey)

	return req, nil
}

func createListAssetsRequest(ctx context.Context, url, username, accessKey, jobID string) (*http.Request, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s/assets", url, username, jobID), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, accessKey)

	return req, nil
}

func createAssetRequest(ctx context.Context, url, username, accessKey, jobID, fileName string) (*http.Request, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s/assets/%s", url, username, jobID, fileName), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, accessKey)

	return req, nil
}

func createStopRequest(ctx context.Context, url, username, accessKey, jobID string) (*http.Request, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s/stop", url, username, jobID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, accessKey)
	return req, nil
}

// FIXME This has nothing to do with resto and belongs to the testcomposer package
func createUploadAssetRequest(ctx context.Context, url, username, accessKey, jobID, fileName, contentType string, content []byte) (*http.Request, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", fileName))
	h.Set("Content-Type", contentType)
	wr, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err = wr.Write(content); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}

	req, err := requesth.NewWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/v1/testcomposer/jobs/%s/assets", url, jobID), &b)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, accessKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

type assetsUploadResponse struct {
	Uploaded []string `json:"uploaded"`
	Errors   []string `json:"errors,omitempty"`
}

func doRequestAsset(httpClient *http.Client, request *http.Request) error {
	resp, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return ErrServerError
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrJobNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("assets upload request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return err
	}

	var assetsResponse assetsUploadResponse
	if err = json.NewDecoder(resp.Body).Decode(&assetsResponse); err != nil {
		return err
	}
	if len(assetsResponse.Errors) > 0 {
		return fmt.Errorf("upload failed: %v", strings.Join(assetsResponse.Errors, ","))
	}
	return nil
}

// DownloadArtifact does downloading artifacts
func (c *Client) DownloadArtifact(jobID string) {
	targetDir := filepath.Join(c.ArtifactConfig.Directory, jobID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Error().Msgf("Unable to create %s to fetch artifacts (%v)", targetDir, err)
		return
	}

	files, err := c.GetJobAssetFileNames(context.Background(), jobID)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return
	}
	for _, f := range files {
		for _, pattern := range c.ArtifactConfig.Match {
			if glob.Glob(pattern, f) {
				if err := c.downloadArtifact(targetDir, jobID, f); err != nil {
					log.Error().Err(err).Msgf("Failed to download file: %s", f)
				}
				break
			}
		}
	}
}

func (c *Client) downloadArtifact(targetDir, jobID, fileName string) error {
	content, err := c.GetJobAssetFileContent(context.Background(), jobID, fileName)
	if err != nil {
		return err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return os.WriteFile(targetFile, content, 0644)
}
