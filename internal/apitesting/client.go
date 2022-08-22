package apitesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

type Client struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

type SyncTestResult struct {
	ID            string  `json:"id,omitempty"`
	FailuresCount int     `json:"failuresCount,omitempty"`
	Project       Project `json:"project,omitempty"`
	Test          Test    `json:"test,omitempty"`
}

type Test struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Project struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func New(url string, username string, accessKey string, timeout time.Duration) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

func (c *Client) RunAllSync(ctx context.Context, hookId string, format string, buildId string) ([]SyncTestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/_run-all-sync?format=%s", c.URL, hookId, format)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []SyncTestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRequest(c.HTTPClient, req)
}

func (c *Client) RunTestSync(ctx context.Context, hookId string, testId string, format string, buildId string) ([]SyncTestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/%s/_run-sync?format=%s", c.URL, hookId, testId, format)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []SyncTestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRequest(c.HTTPClient, req)
}

func (c *Client) RunTagSync(ctx context.Context, hookId string, testTag string, format string, buildId string) ([]SyncTestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/_tag/%s/_run-sync?format=%s", c.URL, hookId, testTag, format)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []SyncTestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRequest(c.HTTPClient, req)
}

func doSyncRequest(client *http.Client, request *http.Request) ([]SyncTestResult, error) {
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return []SyncTestResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return []SyncTestResult{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return []SyncTestResult{}, fmt.Errorf("Test execution failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	testResults := []SyncTestResult{}
	if err := json.NewDecoder(resp.Body).Decode(&testResults); err != nil {
		return []SyncTestResult{}, err
	}

	return testResults, nil
}
