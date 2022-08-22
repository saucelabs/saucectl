package apitesting

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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
	var runResp []SyncTestResult

	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/_run-all-sync?format=%s", c.URL, hookId, format)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return runResp, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRequest(c.HTTPClient, req)
}

func (c *Client) RunTestSync(ctx context.Context, hookId string, testId string, format string, buildId string) ([]SyncTestResult, error) {
	var runResp []SyncTestResult

	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/%s/_run-sync?format=%s", c.URL, hookId, testId, format)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return runResp, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRequest(c.HTTPClient, req)
}

func (c *Client) RunTagSync(ctx context.Context, hookId string, testTag string, format string, buildId string) ([]SyncTestResult, error) {
	var runResp []SyncTestResult

	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/_tag/%s/_run-sync?format=%s", c.URL, hookId, testTag, format)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return runResp, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	return doSyncRequest(c.HTTPClient, req)
}

func doSyncRequest(client *http.Client, request *http.Request) ([]SyncTestResult, error) {
	var runResp []SyncTestResult

	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return runResp, err
	}
	if resp.StatusCode != 200 {
		return runResp, fmt.Errorf("Got a non-200 response: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return runResp, err
	}

	err = json.Unmarshal(body, &runResp)
	if err != nil {
		return runResp, err
	}

	return runResp, nil
}
