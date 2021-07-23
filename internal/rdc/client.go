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
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/ryanuber/go-glob"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/requesth"
	"github.com/saucelabs/saucectl/internal/xcuitest"
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

type readJobScreenshot struct {
	ID string `json:"id,omitempty"`
}

type readJobResponse struct {
	AutomationBackend  string              `json:"automation_backend,omitempty"`
	FrameworkLogURL    string              `json:"framework_log_url,omitempty"`
	DeviceLogURL       string              `json:"device_log_url,omitempty"`
	TestCasesURL       string              `json:"test_cases_url,omitempty"`
	VideoURL           string              `json:"video_url,omitempty"`
	Screenshots        []readJobScreenshot `json:"screenshots,omitempty"`
	Status             string              `json:"status,omitempty"`
	ConsolidatedStatus string              `json:"consolidated_status,omitempty"`
	Error              string              `json:"error,omitempty"`
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
func (c *Client) PollJob(ctx context.Context, id string, interval, timeout time.Duration) (j job.Job, reachedTimeout bool, err error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s", c.URL, id), nil)
	if err != nil {
		return job.Job{}, false, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deathTicker := time.NewTicker(1 * time.Second)
	defer deathTicker.Stop()
	deathclock := time.Now().Add(timeout)

	for {
		select {
		case <-ticker.C:
			j, err = doRequestStatus(c.HTTPClient, req)
			if err != nil {
				return job.Job{}, false, err
			}

			if job.Done(j.Status) {
				j.IsRDC = true
				return j, false, nil
			}
		case <-deathTicker.C:
			if timeout > 0 && time.Now().After(deathclock) {
				return job.Job{}, true, nil
			}
		}
	}
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

// GetJobAssetFileNames returns all assets files available.
func (c *Client) GetJobAssetFileNames(ctx context.Context, jobID string) ([]string, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s", c.URL, jobID), nil)
	if err != nil {
		return []string{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return []string{}, fmt.Errorf("unexpected statusCode: %v", resp.StatusCode)
	}

	var jr readJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return []string{}, err
	}
	return extractAssetsFileNames(jr), nil
}

// extractAssetsFileNames infers available assets from an RDC job.
func extractAssetsFileNames(jr readJobResponse) []string {
	var files []string

	if strings.HasSuffix(jr.DeviceLogURL, "/deviceLogs") {
		files = append(files, "device.log")
	}
	if strings.HasSuffix(jr.VideoURL, "/video.mp4") {
		files = append(files, "video.mp4")
	}
	if len(jr.Screenshots) > 0 {
		files = append(files, "screenshots.zip")
	}

	// xcuitest.log is available for espresso according to API, but will always be empty,
	// => hiding it until API is fixed.
	if jr.AutomationBackend == xcuitest.Kind && strings.HasSuffix(jr.FrameworkLogURL, "/xcuitestLogs") {
		files = append(files, "xcuitest.log")
	}
	// junit.xml is available only for native frameworks.
	if jr.AutomationBackend == xcuitest.Kind || jr.AutomationBackend == espresso.Kind {
		files = append(files, "junit.xml")
	}
	return files
}

// jobURIMappings contains the assets that don't get accessed by their filename.
// Those items also requires to send "Accept: text/plain" header to get raw content instead of json.
var jobURIMappings = map[string]string{
	"device.log":   "deviceLogs",
	"xcuitest.log": "xcuitestLogs",
}

// GetJobAssetFileContent returns the job asset file content.
func (c *Client) GetJobAssetFileContent(ctx context.Context, jobID, fileName string) ([]byte, error) {
	acceptHeader := ""
	URIFileName := fileName
	if _, ok := jobURIMappings[fileName]; ok {
		URIFileName = jobURIMappings[fileName]
		acceptHeader = "text/plain"
	}

	request, err := createAssetRequest(ctx, c.URL, c.Username, c.AccessKey, jobID, URIFileName, acceptHeader)
	if err != nil {
		return nil, err
	}

	data, err := doAssetRequest(c.HTTPClient, request)
	if err != nil {
		return []byte{}, err
	}
	return data, err
}

func createAssetRequest(ctx context.Context, url, username, accessKey, jobID, fileName, acceptHeader string) (*http.Request, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s/%s", url, jobID, fileName), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, accessKey)
	if acceptHeader != "" {
		req.Header.Set("Accept", acceptHeader)
	}
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

type devicesResponse struct {
	Entities []device `json:"entities"`
}

type device struct {
	Name string `json:"name"`
	OS   string `json:"os"`
}

// GetDevices returns the list of available devices using a specific operating system.
func (c *Client) GetDevices(ctx context.Context, OS string) ([]devices.Device, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/rdc/devices/filtered", c.URL), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("os", OS)
	req.URL.RawQuery = q.Encode()
	req.SetBasicAuth(c.Username, c.AccessKey)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return []devices.Device{}, err
	}

	var resp devicesResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return []devices.Device{}, err
	}

	var dev []devices.Device
	for _, d := range resp.Entities {
		dev = append(dev, devices.Device{
			Name: d.Name,
			OS:   d.OS,
		})
	}
	return dev, nil
}
