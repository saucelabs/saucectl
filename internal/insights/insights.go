package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// Client service
type Client struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

var LaunchOptions = map[config.LaunchOrder]string{
	config.LaunchOrderFailRate: "fail_rate",
}

func New(url string, creds credentials.Credentials, timeout time.Duration) Client {
	return Client{
		HTTPClient:  &http.Client{Timeout: timeout},
		URL:         url,
		Credentials: creds,
	}
}

// GetHistory returns job history from insights
func (c *Client) GetHistory(ctx context.Context, user iam.User, launchOrder config.LaunchOrder) (JobHistory, error) {
	start := time.Now().AddDate(0, 0, -7).Unix()
	now := time.Now().Unix()

	var jobHistory JobHistory
	url := fmt.Sprintf("%s/v2/insights/vdc/test-cases", c.URL)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return jobHistory, err
	}

	q := req.URL.Query()
	queries := map[string]string{
		"user_id": user.ID,
		"org_id":  user.Organization.ID,
		"start":   strconv.FormatInt(start, 10),
		"since":   strconv.FormatInt(start, 10),
		"end":     strconv.FormatInt(now, 10),
		"until":   strconv.FormatInt(now, 10),
		"limit":   "200",
		"offset":  "0",
		"sort_by": string(launchOrder),
	}
	for k, v := range queries {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return jobHistory, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return jobHistory, err
	}

	err = json.Unmarshal(body, &jobHistory)
	if err != nil {
		return jobHistory, err
	}
	return jobHistory, nil
}

type testRunsInput struct {
	TestRuns []TestRun `json:"test-runs,omitempty"`
}

// PostTestRun publish test-run results to insights API.
func (c *Client) PostTestRun(ctx context.Context, runs []TestRun) error {
	url := fmt.Sprintf("%s/test-runs/", c.URL)

	input := testRunsInput{
		TestRuns: runs,
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	payloadReader := bytes.NewReader(payload)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	// API Replies 204, doc says 200. Supporting both for now.
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	return errors.New(fmt.Sprintf("Unexpected status code from API: %d", resp.StatusCode))
}
