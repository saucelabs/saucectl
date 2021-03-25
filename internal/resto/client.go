package resto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/requesth"
	"io"
	"net/http"
	"time"

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
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
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
		return  job.Job{}, err
	}
	j, err := doRequest(c.HTTPClient, request)
	if err != nil {
		return job.Job{}, err
	}
	return j, nil
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
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s", url, username, jobID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
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
