package rdc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/requesth"
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

type organizationResponse struct {
	Maximum int `json:"maximum,omitempty"`
}

type concurrencyResponse struct {
	Organization organizationResponse `json:"organization,omitempty"`
}

type testReportResponse struct {
	ID string `json:"id,omitempty"`
}

type startJobResponse struct {
	TestReport testReportResponse `json:"test_report,omitempty"`
}

type readJobResponse struct {
	Status             string `json:"status,omitempty"`
	ConsolidatedStatus string `json:"consolidated_status,omitempty"`
	Error              string `json:"error,omitempty"`
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

// StartJob starts a job on RDC cloud.
func (c *Client) StartJob(options job.RDCStarterOptions) (string, error) {
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(options)

	req, err := requesth.NewWithContext(context.Background(), http.MethodPost,
		fmt.Sprintf("%s/v1/rdc/native-composer/tests", c.URL), &b)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected statusCode: %v", resp.StatusCode)
	}

	var sr startJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", err
	}

	return sr.TestReport.ID, nil
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

	jobDetails := job.Job{}
	if err := json.NewDecoder(resp.Body).Decode(&jobDetails); err != nil {
		return job.Job{}, err
	}

	return jobDetails, nil
}