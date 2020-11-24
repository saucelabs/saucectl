package resto

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	competeJobStatus string = "complete"
	errorJobStatus string = "error"
)

var jobStatuses = map[string]struct{}{
	competeJobStatus: struct {}{},
	errorJobStatus: struct {}{},
}

// ErrServerInaccessible represents error message from server
var ErrServerInaccessible = errors.New("couldn't reach resto server")

// Client http client for getting resto details
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

// GetJobDetails get resto details
func (c *Client) GetJobDetails(id string) (Details, error) {
	request, err := createRequest(c.Host, c.Username, c.AccessKey, id)
	if err != nil {
		return Details{}, err
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return Details{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusInternalServerError {
		return Details{}, ErrServerInaccessible
	}

	details := Details{}
	if err := json.NewDecoder(response.Body).Decode(&details); err != nil {
		return Details{}, err
	}

	return details, nil
}

// GetJobStatus gets resto status
func (c *Client) GetJobStatus(id string, pollDuration time.Duration) (Details, error) {
	request, err := createRequest(c.Host, c.Username, c.AccessKey, id)
	if err != nil {
		return Details{}, err
	}

	ticker := time.NewTicker(pollDuration)
	defer ticker.Stop()

	jobDetails := Details{}

	for range ticker.C {
		response, err := c.HTTPClient.Do(request)
		if err != nil {
			return Details{}, err
		}
		defer response.Body.Close()

		if response.StatusCode >= http.StatusInternalServerError {
			return Details{}, ErrServerInaccessible
		}

		if err := json.NewDecoder(response.Body).Decode(&jobDetails); err != nil {
			return Details{}, err
		}

		if _, ok := jobStatuses[jobDetails.Status]; ok {
			break
		}
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
