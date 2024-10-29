package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/job"
	tunnels "github.com/saucelabs/saucectl/internal/tunnel"
	"github.com/saucelabs/saucectl/internal/vmd"
)

type restoJob struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Passed              bool   `json:"passed"`
	Status              string `json:"status"`
	Error               string `json:"error"`
	Browser             string `json:"browser"`
	BrowserShortVersion string `json:"browser_short_version"`
	BaseConfig          struct {
		DeviceName string `json:"deviceName"`
		// PlatformName is a complex field that requires judicious treatment.
		//  Observed cases:
		//  - Simulators (iOS): "iOS"
		//  - Emulators (Android): "Linux"
		//  - VMs (Windows/Mac): "Windows 11" or "mac 12"
		PlatformName string `json:"platformName"`

		// PlatformVersion refers to the OS version and is only populated for
		// simulators.
		PlatformVersion string `json:"platformVersion"`
	} `json:"base_config"`
	AutomationBackend string `json:"automation_backend"`

	// OS is a combination of the VM's OS name and version. Version is optional.
	OS string `json:"os"`
}

// Resto http client.
type Resto struct {
	Client    *retryablehttp.Client
	URL       string
	Username  string
	AccessKey string
}

type tunnel struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Status   string `json:"status"` // 'new', 'booting', 'deploying', 'halting', 'running', 'terminated'
	TunnelID string `json:"tunnel_identifier"`
}

// NewResto creates a new client.
func NewResto(url, username, accessKey string, timeout time.Duration) Resto {
	return Resto{
		Client:    NewRetryableClient(timeout),
		URL:       url,
		Username:  username,
		AccessKey: accessKey,
	}
}

// Job returns the job details.
func (c *Resto) Job(ctx context.Context, id string, realDevice bool) (job.Job, error) {
	if realDevice {
		return job.Job{}, errors.New("the VDC client does not support real device jobs")
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1.1/%s/jobs/%s", c.URL, c.Username, id), nil)
	if err != nil {
		return job.Job{}, err
	}

	req.Header.Set("Content-Type", "application/json")
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
		err := fmt.Errorf("job status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return job.Job{}, err
	}

	return c.parseJob(resp.Body)
}

// PollJob polls job details at an interval, until timeout has been reached or until the job has ended, whether successfully or due to an error.
func (c *Resto) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
	if realDevice {
		return job.Job{}, errors.New("the VDC client does not support real device jobs")
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
		case <-ticker.C:
			j, err := c.Job(ctx, id, realDevice)
			if err != nil {
				return job.Job{}, err
			}

			if job.Done(j.Status) {
				return j, nil
			}
		case <-deathclock.C:
			j, err := c.Job(ctx, id, realDevice)
			if err != nil {
				return job.Job{}, err
			}
			j.TimedOut = true
			return j, nil
		}
	}
}

// ArtifactNames return the job assets list.
func (c *Resto) ArtifactNames(ctx context.Context, jobID string, realDevice bool) ([]string, error) {
	if realDevice {
		return nil, errors.New("the VDC client does not support real device jobs")
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s/assets", c.URL, c.Username, jobID), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.Do(req)
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
		if k == "video" || k == "screenshots" {
			continue
		}

		if v != nil && reflect.TypeOf(v).Name() == "string" {
			filesList = append(filesList, v.(string))
		}
	}
	return filesList, nil
}

// Artifact returns the job asset file content.
func (c *Resto) Artifact(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error) {
	if realDevice {
		return nil, errors.New("the VDC client does not support real device jobs")
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s/assets/%s", c.URL, c.Username, jobID, fileName), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

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

// IsTunnelRunning checks whether tunnelID is running. If not, it will wait for the tunnel to become available or
// timeout. Whichever comes first.
func (c *Resto) IsTunnelRunning(ctx context.Context, id, owner string, filter tunnels.Filter, wait time.Duration) error {
	deathclock := time.Now().Add(wait)
	var err error
	for time.Now().Before(deathclock) {
		if err = c.isTunnelRunning(ctx, id, owner, filter); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return err
}

func (c *Resto) isTunnelRunning(ctx context.Context, id, owner string, filter tunnels.Filter) error {
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/tunnels", c.URL, c.Username), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	q := req.URL.Query()
	q.Add("full", "true")
	q.Add("all", "true")

	if filter != "" {
		q.Add("filter", string(filter))
	}
	req.URL.RawQuery = q.Encode()

	res, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		err := fmt.Errorf("tunnel request failed; unexpected response code:'%d', msg:'%v'", res.StatusCode, string(body))
		return err
	}

	var resp map[string][]tunnel
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return err
	}

	// Owner should be the current user or the defined parent if there is one.
	if owner == "" {
		owner = c.Username
	}

	for _, tt := range resp {
		for _, t := range tt {
			// User could be using tunnel name (aka tunnel_identifier) or the tunnel ID. Make sure we check both.
			if t.TunnelID != id && t.ID != id {
				continue
			}
			if t.Owner != owner {
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
func (c *Resto) StopJob(ctx context.Context, jobID string, realDevice bool) (job.Job, error) {
	if realDevice {
		return job.Job{}, errors.New("the VDC client does not support real device jobs")
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s/stop", c.URL, c.Username, jobID), nil)
	if err != nil {
		return job.Job{}, err
	}

	req.Header.Set("Content-Type", "application/json")
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
		err := fmt.Errorf("job status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return job.Job{}, err
	}

	return c.parseJob(resp.Body)
}

// GetVirtualDevices returns the list of available virtual devices.
func (c *Resto) GetVirtualDevices(ctx context.Context, kind string) ([]vmd.VirtualDevice, error) {
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/rest/v1.1/info/platforms/all", c.URL), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	res, err := c.Client.Do(req)
	if err != nil {
		return []vmd.VirtualDevice{}, err
	}

	var resp []struct {
		LongName     string `json:"long_name"`
		ShortVersion string `json:"short_version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return []vmd.VirtualDevice{}, err
	}

	key := "Emulator"
	if kind == vmd.IOSSimulator {
		key = "Simulator"
	}

	devs := map[string]map[string]bool{}
	for _, d := range resp {
		if !strings.Contains(d.LongName, key) {
			continue
		}
		if _, ok := devs[d.LongName]; !ok {
			devs[d.LongName] = map[string]bool{}
		}
		devs[d.LongName][d.ShortVersion] = true
	}

	var dev []vmd.VirtualDevice
	for vmdName, versions := range devs {
		d := vmd.VirtualDevice{Name: vmdName}
		for version := range versions {
			d.OSVersion = append(d.OSVersion, version)
		}
		sort.Strings(d.OSVersion)
		dev = append(dev, d)
	}
	sort.Slice(dev, func(i, j int) bool {
		return dev[i].Name < dev[j].Name
	})
	return dev, nil
}

// parseJob parses the body into restoJob and converts it to job.Job.
func (c *Resto) parseJob(body io.ReadCloser) (job.Job, error) {
	var j restoJob
	if err := json.NewDecoder(body).Decode(&j); err != nil {
		return job.Job{}, err
	}

	osName := j.BaseConfig.PlatformName
	osVersion := j.BaseConfig.PlatformVersion

	// PlatformVersion is only populated for simulators. For emulators and VMs,
	// we shall parse the OS field.
	if osVersion == "" {
		segments := strings.Split(j.OS, " ")
		osName = segments[0]
		if len(segments) > 1 {
			osVersion = segments[1]
		}
	}

	return job.Job{
		ID:             j.ID,
		Name:           j.Name,
		Passed:         j.Passed,
		Status:         j.Status,
		Error:          j.Error,
		BrowserName:    j.Browser,
		BrowserVersion: j.BrowserShortVersion,
		DeviceName:     j.BaseConfig.DeviceName,
		Framework:      j.AutomationBackend,
		OS:             osName,
		OSVersion:      osVersion,
	}, nil
}
