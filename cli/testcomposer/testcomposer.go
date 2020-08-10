package testcomposer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"github.com/rs/zerolog/log"
)

// Client service
type Client struct {
	HTTPClient http.Client
	URL        string // https://api.staging.saucelabs.net
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
	BrowserName string   `json:"browserName,omitEmpty"`
	TestName    string   `json:"testName,omitEmpty"`
	Framework   string   `json:"framework,omitEmpty"`
	BuildName   string   `json:"buildName,omitEmpty"`
	Tags        []string `json:"tags,omitEmpty"`
}

func (c *Client) StartJob (ctx context.Context, jobStarterPayload JobStarterPayload) (jobID string, err error) {
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
	req.SetBasicAuth(os.Getenv("SAUCE_USERNAME"), os.Getenv("SAUCE_ACCESS_KEY"))
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	msg := string(body)
	if resp.StatusCode >= 300 {
		log.Error().Int("statusCode", resp.StatusCode).
			Str("message", msg).
			Msg("Failed to start job")
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