package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/slice"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/devices/devicestatus"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

// RDCService http client.
type RDCService struct {
	Client    *retryablehttp.Client
	AppURL    string
	URL       string
	Username  string
	AccessKey string
}

type rdcJob struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	AutomationBackend string `json:"automation_backend,omitempty"`
	FrameworkLogURL   string `json:"framework_log_url,omitempty"`
	DeviceLogURL      string `json:"device_log_url,omitempty"`
	TestCasesURL      string `json:"test_cases_url,omitempty"`
	VideoURL          string `json:"video_url,omitempty"`
	NetworkLogURL     string `json:"network_log_url,omitempty"`
	CrashLogURL       string `json:"crash_log_url,omitempty"`
	Screenshots       []struct {
		ID string
	} `json:"screenshots,omitempty"`
	Status             string `json:"status,omitempty"`
	Passed             bool   `json:"passed,omitempty"`
	ConsolidatedStatus string `json:"consolidated_status,omitempty"`
	Error              string `json:"error,omitempty"`
	OS                 string `json:"os,omitempty"`
	OSVersion          string `json:"os_version,omitempty"`
	DeviceName         string `json:"device_name,omitempty"`
}

// RDCSessionRequest represents the RDC session request.
type RDCSessionRequest struct {
	TestFramework       string            `json:"test_framework,omitempty"`
	AppID               string            `json:"app_id,omitempty"`
	TestAppID           string            `json:"test_app_id,omitempty"`
	XCTestRunFileID     string            `json:"xc_test_run_file,omitempty"`
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
	AppSettings         job.AppSettings   `json:"settings_overwrite,omitempty"`
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

// NewRDCService creates a new client.
func NewRDCService(r region.Region, username, accessKey string, timeout time.Duration) RDCService {
	return RDCService{
		Client:    NewRetryableClient(timeout),
		AppURL:    r.AppBaseURL(),
		URL:       r.APIBaseURL(),
		Username:  username,
		AccessKey: accessKey,
	}
}

// StartJob creates a new job in Sauce Labs.
func (c *RDCService) StartJob(ctx context.Context, opts job.StartOptions) (job.Job, error) {
	url := fmt.Sprintf("%s/v1/rdc/native-composer/tests", c.URL)

	var frameworkName string
	switch opts.Framework {
	case "espresso":
		frameworkName = "ANDROID_INSTRUMENTATION"
	case "xcuitest":
		frameworkName = "XCUITEST"
	case "xctest":
		frameworkName = "XCTEST"
	}

	useTestOrchestrator := false
	if v, ok := opts.TestOptions["useTestOrchestrator"]; ok {
		useTestOrchestrator = fmt.Sprintf("%v", v) == "true"
	}

	jobReq := RDCSessionRequest{
		TestName:            opts.Name,
		AppID:               opts.App,
		TestAppID:           opts.TestApp,
		XCTestRunFileID:     opts.XCTestRunFile,
		OtherApps:           opts.OtherApps,
		TestOptions:         c.formatEspressoArgs(opts.TestOptions),
		TestsToRun:          opts.TestsToRun,
		TestsToSkip:         opts.TestsToSkip,
		DeviceQuery:         c.deviceQuery(opts),
		TestFramework:       frameworkName,
		TunnelName:          opts.Tunnel.Name,
		TunnelOwner:         opts.Tunnel.Owner,
		UseTestOrchestrator: useTestOrchestrator,
		Tags:                opts.Tags,
		Build:               opts.Build,
		RealDeviceKind:      opts.RealDeviceKind,
		AppSettings:         opts.AppSettings,
	}

	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(jobReq)
	if err != nil {
		return job.Job{}, err
	}

	req, err := NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return job.Job{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.HTTPClient.Do(req)
	if err != nil {
		return job.Job{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return job.Job{}, err
	}

	if resp.StatusCode >= 300 {
		err = fmt.Errorf("job start failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, strings.TrimSpace(string(body)))
		return job.Job{}, err
	}

	var sessionStart struct {
		TestReport struct {
			ID string
		} `json:"test_report"`
	}
	if err = json.Unmarshal(body, &sessionStart); err != nil {
		return job.Job{}, fmt.Errorf("job start status unknown: unable to parse server response: %w", err)
	}

	return job.Job{
		ID:     sessionStart.TestReport.ID,
		IsRDC:  true,
		Status: job.StateQueued,
		URL:    fmt.Sprintf("%s/tests/%s", c.AppURL, sessionStart.TestReport.ID),
	}, nil
}

func (c *RDCService) StopJob(ctx context.Context, id string, realDevice bool) (job.Job, error) {
	if !realDevice {
		return job.Job{}, errors.New("the RDC client does not support virtual device jobs")
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/v1/rdc/jobs/%s/stop", c.URL, id), nil)
	if err != nil {
		return job.Job{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.Do(req)
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
		err := fmt.Errorf("unable to stop job: %d - %s", resp.StatusCode, string(body))
		return job.Job{}, err
	}

	// RDC does not return any job details in the response.
	return job.Job{
		ID:     id,
		Status: job.StateInProgress,
		URL:    fmt.Sprintf("%s/tests/%s", c.AppURL, id),
	}, nil
}

// Job returns the job details.
func (c *RDCService) Job(ctx context.Context, id string, realDevice bool) (job.Job, error) {
	if !realDevice {
		return job.Job{}, errors.New("the RDC client does not support virtual device jobs")
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s", c.URL, id), nil)
	if err != nil {
		return job.Job{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.Do(req)
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

	if resp.StatusCode != 200 {
		return job.Job{}, fmt.Errorf("unexpected statusCode: %v", resp.StatusCode)
	}

	return c.parseJob(resp.Body)
}

// PollJob polls job details at an interval, until timeout has been reached or until the job has ended, whether successfully or due to an error.
func (c *RDCService) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
	if !realDevice {
		return job.Job{}, errors.New("the RDC client does not support virtual device jobs")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if timeout <= 0 {
		timeout = 24 * time.Hour
	}
	deathclock := time.NewTimer(timeout)
	defer deathclock.Stop()

	for {
		select {
		case <-ctx.Done():
			return job.Job{ID: id}, ctx.Err()
		case <-ticker.C:
			j, err := c.Job(ctx, id, realDevice)
			if err != nil {
				return job.Job{ID: id}, err
			}

			if job.Done(j.Status) {
				j.IsRDC = true
				return j, nil
			}
		case <-deathclock.C:
			j, err := c.Job(ctx, id, realDevice)
			if err != nil {
				return job.Job{ID: id}, err
			}
			j.TimedOut = true
			return j, nil
		}
	}
}

// ArtifactNames returns all assets files available.
func (c *RDCService) ArtifactNames(ctx context.Context, jobID string, realDevice bool) ([]string, error) {
	if !realDevice {
		return nil, errors.New("the RDC client does not support virtual device jobs")
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s", c.URL, jobID), nil)
	if err != nil {
		return []string{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return []string{}, fmt.Errorf("unexpected statusCode: %v", resp.StatusCode)
	}

	var jr rdcJob
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return []string{}, err
	}

	var files []string

	if strings.HasSuffix(jr.DeviceLogURL, "/deviceLogs") {
		files = append(files, "device.log")
	}
	if strings.HasSuffix(jr.VideoURL, "/video.mp4") {
		files = append(files, "video.mp4")
	}
	if strings.HasSuffix(jr.NetworkLogURL, "/network.har") {
		files = append(files, "network.har")
	}
	if strings.HasSuffix(jr.CrashLogURL, "/crash.json") {
		files = append(files, "crash.json")
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
	return files, nil
}

// Artifact returns the job asset file content.
func (c *RDCService) Artifact(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error) {
	if !realDevice {
		return nil, errors.New("the RDC client does not support virtual device jobs")
	}

	// jobURIMappings contains the assets that don't get accessed by their filename.
	// Those items also requires to send "Accept: text/plain" header to get raw content instead of json.
	var jobURIMappings = map[string]string{
		"device.log":   "deviceLogs",
		"xcuitest.log": "xcuitestLogs",
	}

	acceptHeader := ""
	URIFileName := fileName
	if _, ok := jobURIMappings[fileName]; ok {
		URIFileName = jobURIMappings[fileName]
		acceptHeader = "text/plain"
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/jobs/%s/%s", c.URL, jobID, URIFileName), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	if acceptHeader != "" {
		req.Header.Set("Accept", acceptHeader)
	}
	resp, err := c.Client.Do(req)
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

func (c *RDCService) GetDevices(ctx context.Context) ([]devices.Device, error) {
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/rdc/devices", c.URL), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	res, err := c.Client.Do(req)
	if err != nil {
		return []devices.Device{}, err
	}

	var resp []devices.Device
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return []devices.Device{}, err
	}

	return resp, nil
}

func (c *RDCService) GetDevicesStatuses(ctx context.Context) ([]devices.DeviceStatus, error) {
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/rdc/devices/status", c.URL), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	res, err := c.Client.Do(req)
	if err != nil {
		return []devices.DeviceStatus{}, err
	}

	var resp struct {
		Devices []struct {
			Descriptor string
			State      string
			InUseBy    []struct {
				Username string
			}
			IsPrivateDevice bool
		}
	}

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return []devices.DeviceStatus{}, err
	}

	var result []devices.DeviceStatus
	for _, d := range resp.Devices {
		status, err := devicestatus.StrToStatus(d.State)
		if err != nil {
			return []devices.DeviceStatus{}, err
		}

		var inUseBy []string
		for _, user := range d.InUseBy {
			inUseBy = append(inUseBy, user.Username)
		}

		result = append(result, devices.DeviceStatus{
			ID:              d.Descriptor,
			Status:          status,
			InUseBy:         inUseBy,
			IsPrivateDevice: d.IsPrivateDevice,
		})
	}

	return result, nil
}

// GetDevicesByOS returns the list of available devices using a specific operating system.
func (c *RDCService) GetDevicesByOS(ctx context.Context, OS string) ([]devices.Device, error) {
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/rdc/devices/filtered", c.URL), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("os", OS)
	req.URL.RawQuery = q.Encode()
	req.SetBasicAuth(c.Username, c.AccessKey)

	res, err := c.Client.Do(req)
	if err != nil {
		return []devices.Device{}, err
	}

	var resp struct {
		Entities []struct {
			Name string
			OS   string
		}
	}
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
func (c *RDCService) formatEspressoArgs(options map[string]interface{}) map[string]string {
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

// deviceQuery creates a DeviceQuery from opts.
func (c *RDCService) deviceQuery(opts job.StartOptions) DeviceQuery {
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

// parseJob parses the body into rdcJob and converts it to job.Job.
func (c *RDCService) parseJob(body io.ReadCloser) (job.Job, error) {
	var j rdcJob
	err := json.NewDecoder(body).Decode(&j)
	return job.Job{
		ID:         j.ID,
		Name:       j.Name,
		Error:      j.Error,
		Status:     j.Status,
		Passed:     j.Status == job.StatePassed,
		DeviceName: j.DeviceName,
		Framework:  j.AutomationBackend,
		OS:         j.OS,
		OSVersion:  j.OSVersion,
		IsRDC:      true,
		URL:        fmt.Sprintf("%s/tests/%s", c.AppURL, j.ID),
	}, err
}
