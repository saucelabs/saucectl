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

const (
	completeJobStatus string = "complete"
	errorJobStatus    string = "error"
)

// finalJobStates represents states that a job doesn't transition out of, i.e. once the job is in one of these states,
// it's done.
var finalJobStates = map[string]struct{}{
	completeJobStatus: {},
	errorJobStatus:    {},
}

var (
	// ErrServerInaccessible represents error message when server is inaccessible.
	ErrServerInaccessible = errors.New("couldn't reach resto server")
	// ErrJobNotFound represents error message from server when a job was not found.
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

// PollJob polls a server till the end of the job.
// Stops polling the job when the status will be complete or error.
func (c *Client) PollJob(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
	request, err := createRequest(ctx, c.URL, c.Username, c.AccessKey, id)
	if err != nil {
		return job.Job{}, err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		jobDetails, err := doRequest(c.HTTPClient, request)
		if err != nil {
			return job.Job{}, err
		}

		if _, ok := finalJobStates[jobDetails.Status]; ok {
			return jobDetails, nil
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
		return job.Job{}, ErrServerInaccessible
	}

	if resp.StatusCode == http.StatusNotFound {
		return job.Job{}, ErrJobNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		err := fmt.Errorf("status request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
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
