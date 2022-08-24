package apitesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// RunAllSync sychronously runs all the tests for a project described by hookID.
func (c *Client) RunAllSync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_run-all-sync", hookID), buildID, "json", tunnel)

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRun(c.HTTPClient, req)
}

// RunTestSync sychronously runs a single testID for a project described by hookID.
func (c *Client) RunTestSync(ctx context.Context, hookID string, testID string, buildID string, tunnel config.Tunnel) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/%s/_run-sync", hookID, testID), buildID, "json", tunnel)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRun(c.HTTPClient, req)
}

// RunTagSync sychronously runs the all the tests tagged with testTag for a project described by hookID.
func (c *Client) RunTagSync(ctx context.Context, hookID string, testTag string, buildID string, tunnel config.Tunnel) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_tag/%s/_run-sync", hookID, testTag), buildID, "json", tunnel)

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRun(c.HTTPClient, req)
}

func doSyncRun(client *http.Client, request *http.Request) ([]TestResult, error) {
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return []TestResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return []TestResult{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return []TestResult{}, fmt.Errorf("test execution failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	testResults := []TestResult{}
	if err := json.NewDecoder(resp.Body).Decode(&testResults); err != nil {
		return []TestResult{}, err }

	return testResults, nil
}
