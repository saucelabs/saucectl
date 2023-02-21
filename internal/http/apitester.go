package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/saucelabs/saucectl/internal/apitest"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
)

// APITester describes an interface to the api-testing rest endpoints.
type APITester struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

// PublishedTest describes a published test.
type PublishedTest struct {
	Published apitest.Test
}

// NewAPITester a new instance of APITester.
func NewAPITester(url string, username string, accessKey string, timeout time.Duration) APITester {
	return APITester{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// GetProject returns Project metadata for a given hookID.
func (c *APITester) GetProject(ctx context.Context, hookID string) (apitest.ProjectMeta, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s", c.URL, hookID)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apitest.ProjectMeta{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.ProjectMeta{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.ProjectMeta{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.ProjectMeta{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var project apitest.ProjectMeta
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return project, err
	}
	return project, nil
}

func (c *APITester) GetEventResult(ctx context.Context, hookID string, eventID string) (apitest.TestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/insights/events/%s", c.URL, hookID, eventID)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apitest.TestResult{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.TestResult{}, err
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.TestResult{}, errors.New(msg.InternalServerError)
	}
	// 404 needs to be treated differently to ensure calling parent is aware of the specific error.
	// API replies 404 until the event is fully processed.
	if resp.StatusCode == http.StatusNotFound {
		return apitest.TestResult{}, errors.New("event not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.TestResult{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}
	var testResult apitest.TestResult
	if err := json.NewDecoder(resp.Body).Decode(&testResult); err != nil {
		return testResult, err
	}
	return testResult, nil
}

func (c *APITester) GetTest(ctx context.Context, hookID string, testID string) (apitest.Test, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/%s", c.URL, hookID, testID)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apitest.Test{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.Test{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.Test{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.Test{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var test PublishedTest
	if err := json.NewDecoder(resp.Body).Decode(&test); err != nil {
		return test.Published, err
	}
	return test.Published, nil
}

func (c *APITester) composeURL(path string, buildID string, format string, tunnel config.Tunnel, taskID string) string {
	// NOTE: API url is not user provided so skip error check
	url, _ := url.Parse(c.URL)
	url.Path = path

	query := url.Query()
	if buildID != "" {
		query.Set("buildId", buildID)
	}
	if format != "" {
		query.Set("format", format)
	}

	if tunnel.Name != "" {
		var t string
		if tunnel.Owner != "" {
			t = fmt.Sprintf("%s:%s", tunnel.Owner, tunnel.Name)
		} else {
			t = fmt.Sprintf("%s:%s", c.Username, tunnel.Name)
		}

		query.Set("tunnelId", t)
	}

	if taskID != "" {
		query.Set("taskId", taskID)
	}

	url.RawQuery = query.Encode()

	return url.String()
}

// GetProjects returns the list of Project available.
func (c *APITester) GetProjects(ctx context.Context) ([]apitest.ProjectMeta, error) {
	url := fmt.Sprintf("%s/api-testing/api/project", c.URL)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []apitest.ProjectMeta{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []apitest.ProjectMeta{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return []apitest.ProjectMeta{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return []apitest.ProjectMeta{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%s'", resp.StatusCode, body)
	}

	var projects []apitest.ProjectMeta
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return projects, err
	}
	return projects, nil
}

// GetHooks returns the list of hooks available.
func (c *APITester) GetHooks(ctx context.Context, projectID string) ([]apitest.Hook, error) {
	url := fmt.Sprintf("%s/api-testing/api/project/%s/hook", c.URL, projectID)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []apitest.Hook{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []apitest.Hook{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return []apitest.Hook{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return []apitest.Hook{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%s'", resp.StatusCode, body)
	}

	var hooks []apitest.Hook
	if err := json.NewDecoder(resp.Body).Decode(&hooks); err != nil {
		return hooks, err
	}
	return hooks, nil
}

// RunAllAsync runs all the tests for the project described by hookID and returns without waiting for their results.
func (c *APITester) RunAllAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_run-all", hookID), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	return resp, nil
}

// RunEphemeralAsync runs the tests for the project described by hookID and returns without waiting for their results.
func (c *APITester) RunEphemeralAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, taskID string, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_exec", hookID), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	return resp, nil
}

// RunTestAsync runs a single test described by testID for the project described by hookID and returns without waiting for results.
func (c *APITester) RunTestAsync(ctx context.Context, hookID string, testID string, buildID string, tunnel config.Tunnel, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/%s/_run", hookID, testID), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	return resp, nil
}

// RunTagAsync runs all the tests for a testTag for a project described by hookID and returns without waiting for results.
func (c *APITester) RunTagAsync(ctx context.Context, hookID string, testTag string, buildID string, tunnel config.Tunnel, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_tag/%s/_run", hookID, testTag), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	return resp, nil
}

func (c *APITester) doAsyncRun(client *http.Client, request *http.Request) (apitest.AsyncResponse, error) {
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.AsyncResponse{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.AsyncResponse{}, fmt.Errorf("test execution failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var asyncResponse apitest.AsyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&asyncResponse); err != nil {
		return apitest.AsyncResponse{}, err
	}

	return asyncResponse, nil
}
