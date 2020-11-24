package resto

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	completeJobStatus string = "complete"
	errorJobStatus    string = "error"
)

var jobStatuses = map[string]struct{}{
	completeJobStatus: struct{}{},
	errorJobStatus:    struct{}{},
}

var (
	// ErrServerInaccessible represents error message when server is inaccessible
	ErrServerInaccessible = errors.New("couldn't reach resto server")
	// ErrNotFoundUser represents error message from server when user was not found
	ErrNotFoundUser = errors.New("user was not found")
)

// Client http client
type Client struct {
	HTTPClient *http.Client
	Host       string
	Username   string
	AccessKey  string
}

// New creates a new client
func New(host, username, accessKey string, timeout int) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
		Host:       host,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// GetJobDetails get job details
func (c *Client) GetJobDetails(id string) (Details, error) {
	request, err := createRequest(c.Host, c.Username, c.AccessKey, id)
	if err != nil {
		return Details{}, err
	}

	return makeRequest(c.HTTPClient, request)
}

// GetJobStatus gets job status
func (c *Client) GetJobStatus(id string, pollDuration time.Duration) (Details, error) {
	request, err := createRequest(c.Host, c.Username, c.AccessKey, id)
	if err != nil {
		return Details{}, err
	}

	ticker := time.NewTicker(pollDuration)
	defer ticker.Stop()

	for range ticker.C {
		jobDetails, err := makeRequest(c.HTTPClient, request)
		if err != nil {
			return Details{}, err
		}

		if _, ok := jobStatuses[jobDetails.Status]; ok {
			return jobDetails, nil
		}
	}

	return Details{}, nil
}

func makeRequest(httpClient *http.Client, request *http.Request) (Details, error) {
	response, err := httpClient.Do(request)
	if err != nil {
		return Details{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusInternalServerError {
		return Details{}, ErrServerInaccessible
	}

	if response.StatusCode == http.StatusNotFound {
		return Details{}, ErrNotFoundUser
	}

	jobDetails := Details{}
	if err := json.NewDecoder(response.Body).Decode(&jobDetails); err != nil {
		return Details{}, err
	}

	return jobDetails, nil
}

func createRequest(host, username, accessKey, jobID string) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/rest/v1/%s/jobs/%s", host, username, jobID), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(username, accessKey)

	return request, nil
}
