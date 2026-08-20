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
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/aiauthoring"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/version"
)

const aiAuthoringEntitlementKey = "ai_authoring.enabled"

// AIAuthoringAPIError is a structured error reported by the AI Authoring
// API, e.g. SC_TUNNEL_NOT_FOUND or TEST_CASE_REVISION_NOT_FOUND.
type AIAuthoringAPIError struct {
	StatusCode int
	Code       string
	Detail     string
}

func (e *AIAuthoringAPIError) Error() string {
	if e.Detail != "" && e.Code != "" {
		return fmt.Sprintf("%s (%s)", e.Detail, e.Code)
	}
	if e.Detail != "" {
		return e.Detail
	}
	return fmt.Sprintf("unexpected server response (%d)", e.StatusCode)
}

// AIAuthoringService is a client for the Sauce Labs AI Test Authoring API.
type AIAuthoringService struct {
	// Client is used for idempotent (GET) requests and retries on
	// connection errors, 429 and 5xx responses.
	Client *retryablehttp.Client
	// HTTPClient is used for non-idempotent (POST, DELETE) requests, which
	// are never retried.
	HTTPClient  *http.Client
	URL         string
	Credentials iam.Credentials
}

// NewAIAuthoringService returns a new AIAuthoringService for the given region.
func NewAIAuthoringService(r region.Region, creds iam.Credentials, timeout time.Duration) AIAuthoringService {
	return AIAuthoringService{
		Client: newAIRetryableClient(timeout),
		HTTPClient: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		URL:         r.APIBaseURL(),
		Credentials: creds,
	}
}

// newAIRetryableClient is like NewRetryableClient, but does not retry 404s:
// for test case lookups a 404 is a definitive "not found" answer.
func newAIRetryableClient(timeout time.Duration) *retryablehttp.Client {
	return &retryablehttp.Client{
		HTTPClient: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 10 * time.Second,
		RetryMax:     3,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
		ErrorHandler: retryablehttp.PassthroughErrorHandler,
	}
}

// ListTestCases returns saved test cases matching the given options.
func (c *AIAuthoringService) ListTestCases(ctx context.Context, opts aiauthoring.ListOptions) (aiauthoring.List, error) {
	u := fmt.Sprintf("%s/v1/ai-authoring/testcases", c.URL)
	if q := listQuery(opts); q != "" {
		u += "?" + q
	}

	body, err := c.get(ctx, u)
	if err != nil {
		return aiauthoring.List{}, err
	}

	return parseTestCaseList(body)
}

// GetTestCase fetches a single saved test case by its id.
func (c *AIAuthoringService) GetTestCase(ctx context.Context, id string) (aiauthoring.TestCase, error) {
	u := fmt.Sprintf("%s/v1/ai-authoring/testcases/%s", c.URL, url.PathEscape(id))

	body, err := c.get(ctx, u)
	if err != nil {
		return aiauthoring.TestCase{}, testCaseErr(id, err)
	}

	return parseData[aiauthoring.TestCase](body)
}

// RenameTestCase renames a saved test case.
func (c *AIAuthoringService) RenameTestCase(ctx context.Context, id, name string) (aiauthoring.TestCase, error) {
	u := fmt.Sprintf("%s/v1/ai-authoring/testcases/%s/rename", c.URL, url.PathEscape(id))

	payload, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: name})
	if err != nil {
		return aiauthoring.TestCase{}, err
	}

	body, err := c.do(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return aiauthoring.TestCase{}, testCaseErr(id, err)
	}
	if len(body) == 0 {
		return aiauthoring.TestCase{ID: id, Name: name}, nil
	}

	return parseData[aiauthoring.TestCase](body)
}

// RunTestCase starts a cloud run of a saved test case. The returned run
// holds one ordinary Sauce Labs job per target.
func (c *AIAuthoringService) RunTestCase(ctx context.Context, id string, r aiauthoring.RunRequest) (aiauthoring.TestCaseRun, error) {
	u := fmt.Sprintf("%s/v1/ai-authoring/testcases/%s/run", c.URL, url.PathEscape(id))

	payload, err := json.Marshal(r)
	if err != nil {
		return aiauthoring.TestCaseRun{}, err
	}

	body, err := c.do(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return aiauthoring.TestCaseRun{}, testCaseErr(id, err)
	}

	return parseData[aiauthoring.TestCaseRun](body)
}

// GetTestSuite fetches a single saved test suite by its id.
func (c *AIAuthoringService) GetTestSuite(ctx context.Context, id string) (aiauthoring.TestSuite, error) {
	u := fmt.Sprintf("%s/v1/ai-authoring/testsuites/%s", c.URL, url.PathEscape(id))

	body, err := c.get(ctx, u)
	if err != nil {
		var apiErr *AIAuthoringAPIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return aiauthoring.TestSuite{}, fmt.Errorf("test suite %q not found", id)
		}
		return aiauthoring.TestSuite{}, err
	}

	return parseData[aiauthoring.TestSuite](body)
}

// GetTestCaseRun fetches the current state of a test case run. Job details
// (sauceJobId, url, success) are filled in by the backend as the run
// progresses.
func (c *AIAuthoringService) GetTestCaseRun(ctx context.Context, testCaseID, runID string) (aiauthoring.TestCaseRun, error) {
	u := fmt.Sprintf("%s/v1/ai-authoring/testcases/%s/runs/%s",
		c.URL, url.PathEscape(testCaseID), url.PathEscape(runID))

	body, err := c.get(ctx, u)
	if err != nil {
		return aiauthoring.TestCaseRun{}, err
	}

	return parseData[aiauthoring.TestCaseRun](body)
}

// DeleteTestCase deletes a saved test case.
func (c *AIAuthoringService) DeleteTestCase(ctx context.Context, id string) error {
	u := fmt.Sprintf("%s/v1/ai-authoring/testcases/%s", c.URL, url.PathEscape(id))

	_, err := c.do(ctx, http.MethodDelete, u, nil)
	return testCaseErr(id, err)
}

// testCaseErr converts a generic 404 into a friendly, test-case-specific
// error, preserving the backend's detail when it reported one.
func testCaseErr(id string, err error) error {
	var apiErr *AIAuthoringAPIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		if apiErr.Detail != "" {
			return fmt.Errorf("test case %q: %s", id, apiErr)
		}
		return fmt.Errorf("test case %q not found", id)
	}
	return err
}

// IsAIAuthoringEnabled reports whether the ai_authoring.enabled entitlement
// is granted to the given organization.
func (c *AIAuthoringService) IsAIAuthoringEnabled(ctx context.Context, orgID string) (bool, error) {
	if orgID == "" {
		return false, errors.New("orgID is required to fetch entitlements")
	}
	u := fmt.Sprintf("%s/v2/entitlements/entities/org/%s?entitlements=%s",
		c.URL, url.PathEscape(orgID), url.QueryEscape(aiAuthoringEntitlementKey))

	body, err := c.get(ctx, u)
	if err != nil {
		return false, err
	}

	var resp struct {
		Entitlements []struct {
			Name  string `json:"name"`
			Value any    `json:"value"`
		} `json:"entitlements"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("unexpected entitlements response: %w", err)
	}

	for _, e := range resp.Entitlements {
		if e.Name != aiAuthoringEntitlementKey {
			continue
		}
		switch v := e.Value.(type) {
		case bool:
			return v, nil
		case string:
			return v == "true", nil
		}
	}

	// The entitlement gate fails closed.
	return false, nil
}

// get performs a retryable GET request and returns the response body.
func (c *AIAuthoringService) get(ctx context.Context, url string) ([]byte, error) {
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req.Header)
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readResponse(resp)
}

// do performs a non-retryable request and returns the response body.
func (c *AIAuthoringService) do(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	req, err := NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req.Header)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readResponse(resp)
}

func (c *AIAuthoringService) applyHeaders(h http.Header) {
	// The AI Authoring backend identifies clients by this header.
	h.Set("requested-by", "saucectl/"+version.Version)
	h.Set("Accept", "application/json")
}

func readResponse(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("Sauce Labs rejected the provided credentials (%d); run `saucectl configure` to update them", resp.StatusCode)
	}

	apiErr := AIAuthoringAPIError{StatusCode: resp.StatusCode}
	var envelope struct {
		Error struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		apiErr.Code = envelope.Error.Code
		apiErr.Detail = envelope.Error.Detail
	}
	if apiErr.Detail == "" && len(body) > 0 && resp.StatusCode != http.StatusNotFound {
		apiErr.Detail = string(body)
	}

	return nil, &apiErr
}

func listQuery(opts aiauthoring.ListOptions) string {
	q := url.Values{}
	if opts.Search != "" {
		q.Set("search", opts.Search)
	}
	if opts.Skip > 0 {
		q.Set("skip", strconv.Itoa(opts.Skip))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.StartDate != "" {
		q.Set("startDate", opts.StartDate)
	}
	if opts.EndDate != "" {
		q.Set("endDate", opts.EndDate)
	}
	if opts.UserID != "" {
		q.Set("userId", opts.UserID)
	}
	if opts.TeamID != "" {
		q.Set("teamId", opts.TeamID)
	}
	if opts.TestSuiteID != "" {
		q.Set("testSuiteId", opts.TestSuiteID)
	}
	return q.Encode()
}

// parseTestCaseList normalizes the listing response. The backend returns
// either a bare array or an envelope ({items|results|testcases: [...],
// total|count: n}), optionally nested one level under "data".
func parseTestCaseList(data []byte) (aiauthoring.List, error) {
	var items []aiauthoring.TestCase
	if err := json.Unmarshal(data, &items); err == nil {
		return aiauthoring.List{Items: items, Total: len(items)}, nil
	}

	var env struct {
		Items     []aiauthoring.TestCase `json:"items"`
		Results   []aiauthoring.TestCase `json:"results"`
		TestCases []aiauthoring.TestCase `json:"testcases"`
		Total     *int                   `json:"total"`
		Count     *int                   `json:"count"`
		Data      json.RawMessage        `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return aiauthoring.List{}, fmt.Errorf("unexpected test case list response: %w", err)
	}

	switch {
	case env.Items != nil:
		items = env.Items
	case env.Results != nil:
		items = env.Results
	case env.TestCases != nil:
		items = env.TestCases
	case len(env.Data) > 0:
		return parseTestCaseList(env.Data)
	}

	total := len(items)
	if env.Total != nil {
		total = *env.Total
	} else if env.Count != nil {
		total = *env.Count
	}

	return aiauthoring.List{Items: items, Total: total}, nil
}

// parseData unwraps the optional {"data": {...}} envelope the AI Authoring
// API wraps around single resources.
func parseData[T any](data []byte) (T, error) {
	var env struct {
		Data *T `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err == nil && env.Data != nil {
		return *env.Data, nil
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		var zero T
		return zero, fmt.Errorf("unexpected server response: %w", err)
	}
	return v, nil
}
