package testcomposer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Client service
type Client struct {
	HTTPClient http.Client
	URL        string // e.g.) https://api.<region>.saucelabs.net
}

// Job represents the sauce labs test job.
type Job struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
}

// JobStarterPayload is a JSON object of parameters used to start a session
// from saucectl
type JobStarterPayload struct {
	User        string   `json:"username"`
	AccessKey   string   `json:"accessKey"`
	BrowserName string   `json:"browserName,omitempty"`
	TestName    string   `json:"testName,omitempty"`
	Framework   string   `json:"framework,omitempty"`
	BuildName   string   `json:"buildName,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// StartJob creates a new job in Sauce Labs.
func (c *Client) StartJob(ctx context.Context, jobStarterPayload JobStarterPayload) (jobID string, err error) {
	url := fmt.Sprintf("%s/v1/testcomposer/jobs/", c.URL)
	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(jobStarterPayload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, b)
	if err != nil {
		return
	}
	req.SetBasicAuth(jobStarterPayload.User, jobStarterPayload.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if resp.StatusCode >= 300 {
		err = fmt.Errorf("Failed to start job. statusCode='%d'", resp.StatusCode)
		return "", err
	}
	var job *Job
	err = json.Unmarshal(body, &job)
	if err != nil {
		return
	}

	return job.ID, nil
}
