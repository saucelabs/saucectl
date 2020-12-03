package resto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/job"
	"io/ioutil"
	"net/http"
	"time"
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
