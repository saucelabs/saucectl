package apitesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// Client describes an interface to the api-testing rest endpoints.
type Client struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

// TestResult describes the result from running an api test.
type TestResult struct {
	EventID              string  `json:"_id,omitempty"`
	FailuresCount        int     `json:"failuresCount,omitempty"`
	Project              Project `json:"project,omitempty"`
	Test                 Test    `json:"test,omitempty"`
	ExecutionTimeSeconds int     `json:"executionTimeSeconds,omitempty"`
	Async                bool    `json:"-"`
	TimedOut             bool    `json:"-"`
}

// PublishedTest describes a published test.
type PublishedTest struct {
	Published Test
}

// Test describes a single test.
type Test struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Project describes the metadata for an api testing project.
type Project struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// New returns a apitesting.Client
func New(url string, username string, accessKey string, timeout time.Duration) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// GetProject returns Project metadata for a given hookID.
func (c *Client) GetProject(ctx context.Context, hookID string) (Project, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s", c.URL, hookID)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Project{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Project{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return Project{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Project{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return project, err
	}
	return project, nil
}

func (c *Client) GetEventResult(ctx context.Context, hookID string, eventID string) (TestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/insights/events/%s", c.URL, hookID, eventID)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return TestResult{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return TestResult{}, err
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return TestResult{}, errors.New(msg.InternalServerError)
	}
	// 404 needs to be treated differently to ensure calling parent is aware of the specific error.
	// API replies 404 until the event is fully processed.
	if resp.StatusCode == http.StatusNotFound {
		return TestResult{}, errors.New("event not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TestResult{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}
	var testResult TestResult
	if err := json.NewDecoder(resp.Body).Decode(&testResult); err != nil {
		return testResult, err
	}
	return testResult, nil
}

func (c *Client) GetTest(ctx context.Context, hookID string, testID string) (Test, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/%s", c.URL, hookID, testID)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Test{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Test{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return Test{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Test{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var test PublishedTest
	if err := json.NewDecoder(resp.Body).Decode(&test); err != nil {
		return test.Published, err
	}
	return test.Published, nil
}

func (c *Client) composeURL(path string, buildID string, format string, tunnel config.Tunnel, taskID string) string {
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
		t := tunnel.Name
		if tunnel.Owner != "" {
			t = fmt.Sprintf("%s:%s", t, tunnel.Owner)
		}

		query.Set("tunnelId", t)
	}

	if taskID != "" {
		query.Set("taskId", taskID)
	}

	url.RawQuery = query.Encode()

	return url.String()
}
