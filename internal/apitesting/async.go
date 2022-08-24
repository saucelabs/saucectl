package apitesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// AsyncResponse describes the json response from the async api endpoints.
type AsyncResponse struct {
	ContextIDs []string `json:"contextIds,omitempty"`
	EventIDs   []string `json:"eventIds,omitempty"`
	TaskID     string   `json:"taskId,omitempty"`
	TestIDs    []string `json:"testIds,omitempty"`
}

// RunAllAsync runs all the tests for the project described by hookID and returns without waiting for their results.
func (c *Client) RunAllAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_run-all", hookID), buildID, "", tunnel)

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return []TestResult{}, err
	}

	apitestProject, err := c.GetProject(ctx, hookID)

	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch project details; go to your project dashboard for test results")
	}

	var testResults []TestResult

	for _, e := range resp.EventIDs {
		testResults = append(testResults, TestResult{
			EventID: e,
			Project: apitestProject,
			Async:   true,
		})
	}
	return testResults, nil
}

// RunTestAsync runs a single test described by testID for the project described by hookID and returns without waiting for results.
func (c *Client) RunTestAsync(ctx context.Context, hookID string, testID string, buildID string, tunnel config.Tunnel) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/%s/_run", hookID, testID), buildID, "", tunnel)

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return []TestResult{}, err
	}

	apitestProject, err := c.GetProject(ctx, hookID)

	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch project details; go to your project dashboard for test results")
	}

	var testResults []TestResult

	for _, e := range resp.EventIDs {
		testResults = append(testResults, TestResult{
			EventID: e,
			Project: apitestProject,
			Async:   true,
		})
	}
	return testResults, nil
}

// RunTagAsync runs all the tests for a testTag for a project described by hookID and returns without waiting for results.
func (c *Client) RunTagAsync(ctx context.Context, hookID string, testTag string, buildID string, tunnel config.Tunnel) ([]TestResult, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_tag/%s/_run", hookID, testTag), buildID, "", tunnel)

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return []TestResult{}, err
	}

	apitestProject, err := c.GetProject(ctx, hookID)

	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch project details; go to your project dashboard for test results")
	}

	var testResults []TestResult

	for _, e := range resp.EventIDs {
		testResults = append(testResults, TestResult{
			EventID: e,
			Project: apitestProject,
			Async:   true,
		})
	}
	return testResults, nil
}

func doAsyncRun(client *http.Client, request *http.Request) (AsyncResponse, error) {
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return AsyncResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return AsyncResponse{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return AsyncResponse{}, fmt.Errorf("test execution failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	asyncResponse := AsyncResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&asyncResponse); err != nil {
		return AsyncResponse{}, err
	}

	return asyncResponse, nil
}
