package rdc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/slice"
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
	NativeClient   *http.Client
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

// SessionRequest represents the RDC session request.
type SessionRequest struct {
	TestFramework       string            `json:"test_framework,omitempty"`
	AppID               string            `json:"app_id,omitempty"`
	TestAppID           string            `json:"test_app_id,omitempty"`
	OtherApps           []string          `json:"other_apps,omitempty"`
	DeviceQuery         DeviceQuery       `json:"device_query,omitempty"`
	TestOptions         map[string]string `json:"test_options,omitempty"`
	TestsToRun          []string          `json:"tests_to_run,omitempty"`
	TestsToSkip         []string          `json:"tests_to_skip,omitempty"`
	TestName            string            `json:"test_name,omitempty"`
	TunnelName          string            `json:"tunnel_name,omitempty"`
	TunnelOwner         string            `json:"tunnel_owner,omitempty"`
	UseTestOrchestrator bool              `json:"use_test_orchestrator,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
	Build               string            `json:"build,omitempty"`
	AppSettings         job.AppSettings   `json:"settings,omitempty"`
	RealDeviceKind      string            `json:"kind,omitempty"`
}

// DeviceQuery represents the device selection query for RDC.
type DeviceQuery struct {
	Type                         string `json:"type"`
	DeviceDescriptorID           string `json:"device_descriptor_id,omitempty"`
	PrivateDevicesOnly           bool   `json:"private_devices_only,omitempty"`
	CarrierConnectivityRequested bool   `json:"carrier_connectivity_requested,omitempty"`
	RequestedDeviceType          string `json:"requested_device_type,omitempty"`
	DeviceName                   string `json:"device_name,omitempty"`
	PlatformVersion              string `json:"platform_version,omitempty"`
}

type sessionStartResponse struct {
	TestReport struct {
		ID string `json:"id"`
	} `json:"test_report"`
}

// New creates a new client.
func New(url, username, accessKey string, timeout time.Duration, artifactConfig config.ArtifactDownload) Client {
	nativeClient := &http.Client{Timeout: timeout}
	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient = nativeClient
	httpClient.Logger = nil
	httpClient.RetryMax = retryMax

	return Client{
		HTTPClient:     httpClient,
		NativeClient:   nativeClient,
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

// StartJob creates a new job in Sauce Labs.
func (c *Client) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
	url := fmt.Sprintf("%s/v1/rdc/native-composer/tests", c.URL)

	var frameworkName string
	switch opts.Framework {
	case "espresso":
		frameworkName = "ANDROID_INSTRUMENTATION"
	case "xcuitest":
		frameworkName = "XCUITEST"
	}

	useTestOrchestrator := false
	if v, ok := opts.TestOptions["useTestOrchestrator"]; ok {
		useTestOrchestrator = fmt.Sprintf("%v", v) == "true"
	}

	jobReq := SessionRequest{
		TestName:            opts.Name,
		AppID:               opts.App,
		TestAppID:           opts.Suite,
		OtherApps:           opts.OtherApps,
		TestOptions:         formatEspressoArgs(opts.TestOptions),
		TestsToRun:          opts.TestsToRun,
		TestsToSkip:         opts.TestsToSkip,
		DeviceQuery:         prepareDeviceQuery(opts),
		TestFramework:       frameworkName,
		TunnelName:          opts.Tunnel.ID,
		TunnelOwner:         opts.Tunnel.Parent,
		UseTestOrchestrator: useTestOrchestrator,
		Tags:                opts.Tags,
		Build:               opts.Build,
		RealDeviceKind:      opts.RealDeviceKind,
		AppSettings:         opts.AppSettings,
	}

	var b bytes.Buffer
	err = json.NewEncoder(&b).Encode(jobReq)
	if err != nil {
		return
	}

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.NativeClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode >= 300 {
		err = fmt.Errorf("job start failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, strings.TrimSpace(string(body)))
		return "", true, err
	}

	var sessionStart sessionStartResponse
	if err = json.Unmarshal(body, &sessionStart); err != nil {
		return "", true, fmt.Errorf("job start status unknown: unable to parse server response: %w", err)
	}

	return sessionStart.TestReport.ID, true, nil
}

// ReadJob returns the job details.
func (c *Client) ReadJob(ctx context.Context, id string, realDevice bool) (job.Job, error) {
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
func (c *Client) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
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
			j, err := doRequestStatus(c.HTTPClient, req)
			if err != nil {
				return job.Job{}, err
			}

			if job.Done(j.Status) {
				j.IsRDC = true
				return j, nil
			}
		case <-deathclock.C:
			j, err := doRequestStatus(c.HTTPClient, req)
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
func (c *Client) GetJobAssetFileNames(ctx context.Context, jobID string, realDevice bool) ([]string, error) {
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
func (c *Client) GetJobAssetFileContent(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error) {
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

// DownloadArtifact does downloading artifacts and returns downloaded file list
func (c *Client) DownloadArtifact(jobID, suiteName string) []string {
	targetDir, err := download.GetDirName(suiteName, c.ArtifactConfig)
	if err != nil {
		log.Error().Msgf("Unable to create artifacts folder (%v)", err)
		return []string{}
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Error().Msgf("Unable to create %s to fetch artifacts (%v)", targetDir, err)
		return []string{}
	}

	files, err := c.GetJobAssetFileNames(context.Background(), jobID, false)
	if err != nil {
		log.Error().Msgf("Unable to fetch artifacts list (%v)", err)
		return []string{}
	}

	filepaths := fpath.MatchFiles(files, c.ArtifactConfig.Match)
	var artifacts []string
	for _, f := range filepaths {
		targetFile, err := c.downloadArtifact(targetDir, jobID, f)
		if err != nil {
			log.Err(err).Msg("Unable to download artifacts")
			return artifacts
		}
		artifacts = append(artifacts, targetFile)
	}

	return artifacts
}

func (c *Client) downloadArtifact(targetDir, jobID, fileName string) (string, error) {
	content, err := c.GetJobAssetFileContent(context.Background(), jobID, fileName, false)
	if err != nil {
		return "", err
	}
	targetFile := filepath.Join(targetDir, fileName)
	return targetFile, os.WriteFile(targetFile, content, 0644)
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

// formatEspressoArgs adapts option shape to match RDC expectations
func formatEspressoArgs(options map[string]interface{}) map[string]string {
	mappedOptions := map[string]string{}
	for k, v := range options {
		if v == nil {
			continue
		}
		// We let the user set 'useTestOrchestrator' inside TestOptions, but RDC has a dedicated setting for it.
		if k == "useTestOrchestrator" {
			continue
		}

		value := fmt.Sprintf("%v", v)

		// class/notClass need special treatment, because we accept these as slices, but the backend wants
		// a comma separated string.
		if k == "class" || k == "notClass" {
			value = slice.Join(v, ",")
		}

		if value == "" {
			continue
		}
		mappedOptions[k] = value
	}
	return mappedOptions
}

// prepareDeviceQuery prepares the DeviceQuery according jobs requirements.
func prepareDeviceQuery(opts job.StartOptions) DeviceQuery {
	if opts.DeviceID != "" {
		return DeviceQuery{
			Type:               "HardcodedDeviceQuery",
			DeviceDescriptorID: opts.DeviceID,
		}
	}
	return DeviceQuery{
		Type:                         "DynamicDeviceQuery",
		CarrierConnectivityRequested: opts.DeviceHasCarrier,
		DeviceName:                   opts.DeviceName,
		PlatformVersion:              opts.PlatformVersion,
		PrivateDevicesOnly:           opts.DevicePrivateOnly,
		RequestedDeviceType:          opts.DeviceType,
	}
}
