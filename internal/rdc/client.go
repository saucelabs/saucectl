package rdc

import (
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

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

var (
	// ErrServerError is returned when the server was not able to correctly handle our request (status code >= 500).
	ErrServerError = errors.New(msg.InternalServerError)
	// ErrJobNotFound is returned when the requested job was not found.
	ErrJobNotFound = errors.New(msg.JobNotFound)
	// ErrAssetNotFound is returned when the requested asset is not found.
	ErrAssetNotFound = errors.New(msg.AssetNotFound)
)

// getStatusMaxRetry is the total retry times when pulling job status
const retryMax = 3

// Client http client.
type Client struct {
	HTTPClient     *retryablehttp.Client
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
	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient = &http.Client{Timeout: timeout}
	httpClient.Logger = nil
	httpClient.RetryMax = retryMax

	return Client{
		HTTPClient:     httpClient,
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

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return 0, err
	}

	resp, err := c.HTTPClient.Do(r)
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

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return job.Job{}, err
	}

	resp, err := c.HTTPClient.Do(r)
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

// PollJob polls job details at an interval, until timeout has been reached or until the job has ended, whether successfully or due to an error.
func (c *Client) PollJob(ctx context.Context, id string, interval, timeout time.Duration) (j job.Job, err error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s", c.URL, id), nil)
	if err != nil {
		return job.Job{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if timeout <= 0 {
		timeout = 24 * time.Hour
	}
	deathclock := time.NewTimer(timeout)
	defer deathclock.Stop()

	for {
		select {
		case <-ticker.C:
			j, err = doRequestStatus(c.HTTPClient, req)
			if err != nil {
				return job.Job{}, err
			}

			if job.Done(j.Status) {
				j.IsRDC = true
				return j, nil
			}
		case <-deathclock.C:
			j, err = doRequestStatus(c.HTTPClient, req)
			if err != nil {
				return job.Job{}, err
			}
			j.TimedOut = true
			return j, nil
		}
	}
}

func doRequestStatus(httpClient *retryablehttp.Client, request *http.Request) (job.Job, error) {
	retryRep, err := retryablehttp.FromRequest(request)
	if err != nil {
		return job.Job{}, err
	}
	resp, err := httpClient.Do(retryRep)
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

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return []string{}, err
	}

	resp, err := c.HTTPClient.Do(r)
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

func doAssetRequest(httpClient *retryablehttp.Client, request *http.Request) ([]byte, error) {
	req, err := retryablehttp.FromRequest(request)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
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

// DownloadArtifact does downloading artifacts
func (c *Client) DownloadArtifact(jobID, suiteName string) {
	targetDir, err := download.GetDirName(suiteName, c.ArtifactConfig)
	if err != nil {
		log.Error().Msgf("Unable to create artifacts folder (%v)", err)
		return
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Error().Msgf("Unable to create %s to fetch artifacts (%v)", targetDir, err)
		return
	}
	files, err := c.GetJobAssetFileNames(context.Background(), jobID)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return
	}
	filepaths := fpath.MatchFiles(files, c.ArtifactConfig.Match)
	for _, f := range filepaths {
		if err := c.downloadArtifact(targetDir, jobID, f); err != nil {
			log.Err(err).Msg("Unable to download artifacts")
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

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	res, err := c.HTTPClient.Do(r)
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
