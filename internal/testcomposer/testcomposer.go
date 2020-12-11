package testcomposer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/internal/fleet"
	"github.com/saucelabs/saucectl/internal/job"
	"io/ioutil"
	"net/http"
)

// forbiddenPreviewError contains the message send by test-composer when access is restricted
const forbiddenPreviewError = "Forbidden: not part of preview"

// Client service
type Client struct {
	HTTPClient  *http.Client
	URL         string // e.g.) https://api.<region>.saucelabs.net
	Credentials credentials.Credentials
}

// Job represents the sauce labs test job.
type Job struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
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
func (c *Client) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, err error) {
	url := fmt.Sprintf("%s/v1/testcomposer/jobs", c.URL)

	var b bytes.Buffer
	err = json.NewEncoder(&b).Encode(opts)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return
	}
	req.SetBasicAuth(opts.User, opts.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// Check if error is related to preview
	if resp.StatusCode == http.StatusForbidden && string(body) == forbiddenPreviewError {
		log.Error().Msg("Looks like you are not part of the preview. To join the preview, please sign up here: https://info.saucelabs.com/scale-cypress-testing.html")
		err = fmt.Errorf("job start failed; not part of preview")
		return "", err
	}

	if resp.StatusCode >= 300 {
		err = fmt.Errorf("job start failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
		return "", err
	}

	var j Job
	err = json.Unmarshal(body, &j)
	if err != nil {
		return
	}

	return j.ID, nil
}

// Register registers a fleet with the given buildID and test suites.
// Returns a fleet ID if successful.
func (c *Client) Register(ctx context.Context, buildID string, testSuites []fleet.TestSuite) (string, error) {
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
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
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
