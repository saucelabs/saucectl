package apitesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

func (c *Client) RunAllSync(ctx context.Context, hookId string, buildId string) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_run-all-sync", hookId), buildId, "json")

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRun(c.HTTPClient, req)
}

func (c *Client) RunTestSync(ctx context.Context, hookId string, testId string, buildId string) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/%s/_run-sync", hookId, testId), buildId, "json")
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRun(c.HTTPClient, req)
}

func (c *Client) RunTagSync(ctx context.Context, hookId string, testTag string, buildId string) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_tag/%s/_run-sync", hookId, testTag), buildId, "json")

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
		return []TestResult{}, fmt.Errorf("Test execution failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	testResults := []TestResult{}
	if err := json.NewDecoder(resp.Body).Decode(&testResults); err != nil {
		return []TestResult{}, err }

	return testResults, nil
}
