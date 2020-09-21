package testcomposer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/internal/fleet"
	"io/ioutil"
	"net/http"
)

// Client service
type Client struct {
	HTTPClient *http.Client
	URL        string // e.g.) https://api.<region>.saucelabs.net
	Username   string
	AccessKey  string
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

// CreatorRequest represents the request body for creating a fleet.
type CreatorRequest struct {
	BuildID    string            `json:"buildID"`
	TestSuites []fleet.TestSuite `json:"testSuites"`
}

// CreatorResponse represents the response body for creating a fleet.
type CreatorResponse struct {
	FleetID string `json:"fleetID"`
}

// AssignerRequest represents the request body for fleet assignments.
type AssignerRequest struct {
	SuiteName string `json:"suiteName"`
}

// AssignerResponse represents the response body for fleet assignments.
type AssignerResponse struct {
	TestFile string `json:"testFile"`
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

// CreateFleet creates a fleet with the given buildID and test suites.
// Returns a fleet ID if successful.
func (c *Client) CreateFleet(ctx context.Context, buildID string, testSuites []fleet.TestSuite) (string, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/fleets", c.URL)

	req, err := c.newJSONRequest(ctx, url, http.MethodPut, CreatorRequest{
		BuildID:    buildID,
		TestSuites: testSuites,
	})
	if err != nil {
		return "", err
	}

	var resp CreatorResponse
	if err := c.doJSONResponse(req, 201, &resp); err != nil {
		return "", err
	}

	return resp.FleetID, nil
}

// NextAssignment fetches the next test assignment based on the suiteName and fleetID.
// Returns an empty string if all tests have been assigned.
func (c *Client) NextAssignment(ctx context.Context, fleetID, suiteName string) (string, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/fleets/%s/assignments/_next", c.URL, fleetID)

	req, err := c.newJSONRequest(ctx, url, http.MethodPut, AssignerRequest{SuiteName: suiteName})
	if err != nil {
		return "", err
	}

	var resp AssignerResponse
	if err := c.doJSONResponse(req, 200, &resp); err != nil {
		return "", err
	}

	return resp.TestFile, nil
}

func (c *Client) newJSONRequest(ctx context.Context, url, method string, payload interface{}) (*http.Request, error) {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(payload); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &b)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)
	req.Header.Set("Content-Type", "application/json")

	return req, err
}

func (c *Client) doJSONResponse(req *http.Request, expectStatus int, v interface{}) error {
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != expectStatus {
		return fmt.Errorf("unexpected response from test-composer: %d", res.StatusCode)
	}

	return json.NewDecoder(res.Body).Decode(v)
}
