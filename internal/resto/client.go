package resto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
)

var (
	// ErrServerError is returned when the server was not able to correctly handle our request (status code >= 500).
	ErrServerError = errors.New("internal server error")
	// ErrJobNotFound is returned when the requested job was not found.
	ErrJobNotFound = errors.New("job was not found")
)

// Client http client.
type Client struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

// concurrencyResponse is the response body as is returned by resto's rest/v1.2/users/{username}/concurrency
type concurrencyResponse struct {
	Concurrency struct {
		Organization struct {
			Allowed struct {
				VMS    int `json:"vms"`
				RDS    int `json:"rds"`
			}
		}
	}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
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
		body, _ := ioutil.ReadAll(resp.Body)
		err := fmt.Errorf("job status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return nil, err
	}

	return ioutil.ReadAll(resp.Body)
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
		body, _ := ioutil.ReadAll(resp.Body)
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s", url, username, jobID), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(username, accessKey)

	return request, nil
}

func createAssetRequest(ctx context.Context, url, username, accessKey, jobID, fileName string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s/assets/%s", url, username, jobID, fileName), nil)
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(username, accessKey)

	return request, nil
}
