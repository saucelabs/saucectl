package rdc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/ryanuber/go-glob"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/requesth"
)

var (
	// ErrServerError is returned when the server was not able to correctly handle our request (status code >= 500).
	ErrServerError = errors.New("internal server error")
	// ErrJobNotFound is returned when the requested job was not found.
	ErrJobNotFound = errors.New("job was not found")
	// ErrAssetNotFound is returned when the requested asset is not found.
	ErrAssetNotFound = errors.New("asset not found")
)

// Client http client.
type Client struct {
	HTTPClient     *http.Client
	URL            string
	Username       string
	AccessKey      string
	ArtifactConfig config.ArtifactDownload
}

type organizationResponse struct {
	Maximum int `json:"maximum,omitempty"`
}

type concurrencyResponse struct {
	Organization organizationResponse `json:"organization,omitempty"`
}

type readJobResponse struct {
	Status             string `json:"status,omitempty"`
	ConsolidatedStatus string `json:"consolidated_status,omitempty"`
	Error              string `json:"error,omitempty"`
}

// New creates a new client.
func New(url, username, accessKey string, timeout time.Duration, artifactConfig config.ArtifactDownload) Client {
	return Client{
		HTTPClient:     &http.Client{Timeout: timeout},
		URL:            url,
		Username:       username,
		AccessKey:      accessKey,
		ArtifactConfig: artifactConfig,
	}
}

// ReadAllowedCCY returns the allowed (max) concurrency for the current account.
func (c *Client) ReadAllowedCCY(ctx context.Context) (int, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/concurrency", c.URL), nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("unexpected statusCode: %v", resp.StatusCode)
	}

	var cr concurrencyResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return 0, err
	}
	return cr.Organization.Maximum, nil
}

// ReadJob returns the job details.
func (c *Client) ReadJob(ctx context.Context, id string) (job.Job, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s", c.URL, id), nil)
	if err != nil {
		return job.Job{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return job.Job{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return job.Job{}, fmt.Errorf("unexpected statusCode: %v", resp.StatusCode)
	}

	var jr readJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return job.Job{}, err
	}
	return job.Job{
		ID:     id,
		Error:  jr.Error,
		Status: jr.Status,
		Passed: jr.Status == job.StatePassed,
	}, nil
}

// PollJob polls job details at an interval, until the job has ended, whether successfully or due to an error.
func (c *Client) PollJob(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s", c.URL, id), nil)
	if err != nil {
		return job.Job{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		j, err := doRequestStatus(c.HTTPClient, req)
		if err != nil {
			return job.Job{}, err
		}

		if job.Done(j.Status) {
			j.IsRDC = true
			return j, nil
		}
	}

	return job.Job{}, nil
}

func doRequestStatus(httpClient *http.Client, request *http.Request) (job.Job, error) {
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

	var jobDetails job.Job
	if err := json.NewDecoder(resp.Body).Decode(&jobDetails); err != nil {
		return job.Job{}, err
	}

	return jobDetails, nil
}

// jobAssetsList represents known assets. As file list is fixed from API perspective, the list must be hardcoded.
var jobAssetsList = []string{"junit.xml", "video.mp4", "deviceLogs", "screenshots.zip"}

// GetJobAssetFileNames returns all assets files available.
func (c *Client) GetJobAssetFileNames(ctx context.Context, jobID string) ([]string, error) {
	return jobAssetsList, nil
}

// GetJobAssetFileContent returns the job asset file content.
func (c *Client) GetJobAssetFileContent(ctx context.Context, jobID, fileName string) ([]byte, error) {
	if !jobAssetsAvailable(fileName) {
		return []byte{}, fmt.Errorf("asset '%s' not available", fileName)
	}
	request, err := createAssetRequest(ctx, c.URL, c.Username, c.AccessKey, jobID, fileName)
	if err != nil {
		return nil, err
	}

	data, err := doAssetRequest(c.HTTPClient, request)
	if err != nil {
		return []byte{}, err
	}

	if fileName == "deviceLogs" {
		return convertDeviceLogs(data)
	}
	return data, err
}

func createAssetRequest(ctx context.Context, url, username, accessKey, jobID, fileName string) (*http.Request, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s/%s", url, jobID, fileName), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, accessKey)

	return req, nil
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
		return nil, ErrAssetNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("job status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

func jobAssetsAvailable(asset string) bool {
	for _, a := range jobAssetsList {
		if a == asset {
			return true
		}
	}
	return false
}

// deviceLogLine represent a line from device console.
type deviceLogLine struct {
	ID      int    `json:"id,omitempty"`
	Time    string `json:"time,omitempty"`
	Level   string `json:"level,omitempty"`
	Message string `json:"message,omitempty"`
}

type deviceLogLines []deviceLogLine

// As device logs are represented as an JSON list, we convert them to
// raw text as it would be when looking device logs.
func convertDeviceLogs(data []byte) ([]byte, error) {
	var lines deviceLogLines
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&lines)
	if err != nil {
		return []byte{}, err
	}

	var b bytes.Buffer
	for _, line := range lines {
		b.Write([]byte(fmt.Sprintf("%s %s %d %s\n", line.Level, line.Time, line.ID, line.Message)))
	}
	return b.Bytes(), nil
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
