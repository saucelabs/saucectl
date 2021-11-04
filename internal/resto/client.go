package resto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
	"github.com/ryanuber/go-glob"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/requesth"
	"github.com/saucelabs/saucectl/internal/vmd"
)

var (
	// ErrServerError is returned when the server was not able to correctly handle our request (status code >= 500).
	ErrServerError = errors.New("internal server error")
	// ErrJobNotFound is returned when the requested job was not found.
	ErrJobNotFound = errors.New("job was not found")
	// ErrTunnelNotFound is returned when the requested tunnel was not found.
	ErrTunnelNotFound = errors.New("tunnel not found")
)

// retryMax is the total retry times when pulling job status
const retryMax = 3

// Client http client.
type Client struct {
	HTTPClient     *retryablehttp.Client
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

// availableTunnelsResponse is the response body as is returned by resto's rest/v1/users/{username}/tunnels endpoint.
type availableTunnelsResponse map[string][]tunnel

type tunnel struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Status   string `json:"status"` // 'new', 'booting', 'deploying', 'halting', 'running', 'terminated'
	TunnelID string `json:"tunnel_identifier"`
}

// New creates a new client.
func New(url, username, accessKey string, timeout time.Duration) Client {
	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient = &http.Client{Timeout: timeout}
	httpClient.Logger = nil
	httpClient.RetryMax = retryMax
	return Client{
		HTTPClient: httpClient,
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

// PollJob polls job details at an interval, until timeout has been reached or until the job has ended, whether successfully or due to an error.
func (c *Client) PollJob(ctx context.Context, id string, interval, timeout time.Duration) (j job.Job, err error) {
	request, err := createRequest(ctx, c.URL, c.Username, c.AccessKey, id)
	if err != nil {
		return job.Job{}, err
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
			j, err = doRequest(c.HTTPClient, request)
			if err != nil {
				return job.Job{}, err
			}

			if job.Done(j.Status) {
				return j, nil
			}
		case <-deathclock.C:
			j, err = doRequest(c.HTTPClient, request)
			if err != nil {
				return job.Job{}, err
			}
			j.TimedOut = true
			return j, nil
		}
	}
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

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return 0, err
	}

	resp, err := c.HTTPClient.Do(r)
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
func (c *Client) IsTunnelRunning(ctx context.Context, id, owner string, wait time.Duration) error {
	deathclock := time.Now().Add(wait)
	var err error
	for time.Now().Before(deathclock) {
		if err = c.isTunnelRunning(ctx, id, owner); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return err
}

func (c *Client) isTunnelRunning(ctx context.Context, id, owner string) error {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/tunnels", c.URL, c.Username), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	q := req.URL.Query()
	q.Add("full", "true")
	q.Add("all", "true")
	req.URL.RawQuery = q.Encode()

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return err
	}

	res, err := c.HTTPClient.Do(r)
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

func doListAssetsRequest(httpClient *retryablehttp.Client, request *http.Request) ([]string, error) {
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
		return nil, ErrJobNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("job status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

func doRequest(httpClient *retryablehttp.Client, request *http.Request) (job.Job, error) {
	req, err := retryablehttp.FromRequest(request)
	if err != nil {
		return job.Job{}, err
	}
	resp, err := httpClient.Do(req)
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

type platformEntry struct {
	LongName     string `json:"long_name"`
	ShortVersion string `json:"short_version"`
}

// GetVirtualDevices returns the list of available virtual devices.
func (c *Client) GetVirtualDevices(ctx context.Context, kind string) ([]vmd.VirtualDevice, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/rest/v1.1/info/platforms/all", c.URL), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return []vmd.VirtualDevice{}, err
	}

	res, err := c.HTTPClient.Do(r)
	if err != nil {
		return []vmd.VirtualDevice{}, err
	}

	var resp []platformEntry
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
