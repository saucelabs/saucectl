package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/authoring"
)

// basePath is the root path of the Test Authoring API.
const authoringBasePath = "/ai-authoring/v1"

// Compile-time check that AuthoringService satisfies authoring.Service.
var _ authoring.Service = (*AuthoringService)(nil)

// AuthoringService implements authoring.Service against the Sauce Labs Test
// Authoring API.
type AuthoringService struct {
	HTTPClient *retryablehttp.Client
	URL        string // e.g. https://api.<region>.saucelabs.com
	Username   string
	AccessKey  string
}

// NewAuthoringService returns a new AuthoringService.
func NewAuthoringService(url, username, accessKey string, timeout time.Duration) *AuthoringService {
	return &AuthoringService{
		HTTPClient: NewRetryableClient(timeout),
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

func (s *AuthoringService) newRequest(ctx context.Context, method, path string, body interface{}) (*retryablehttp.Request, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}

	req, err := NewRetryableRequestWithContext(ctx, method, fmt.Sprintf("%s%s%s", s.URL, authoringBasePath, path), reader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.Username, s.AccessKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// authoringError inspects an error response, trying to extract a useful
// message, and always returns a non-nil error.
func authoringError(resp *http.Response) error {
	var errResp struct {
		Error struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
		} `json:"error"`
	}
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Detail != "" {
		return fmt.Errorf("authoring API error (%d, %s): %s", resp.StatusCode, errResp.Error.Code, errResp.Error.Detail)
	}

	return fmt.Errorf("unexpected authoring API response (%d): %s", resp.StatusCode, string(body))
}

// generateRequestBody mirrors authoring.GenerateRequest's JSON shape.
type generateRequestBody struct {
	Name           string                   `json:"name"`
	TestSuiteID    string                   `json:"testSuiteId,omitempty"`
	Tags           []string                 `json:"tags,omitempty"`
	RunSettings    runSettingsBody          `json:"runSettings"`
	PromptSettings authoring.PromptSettings `json:"promptSettings"`
	Timeout        int                      `json:"timeout,omitempty"`
}

type runSettingsBody struct {
	TestURL      string                 `json:"testUrl,omitempty"`
	SCTunnelName string                 `json:"scTunnelName,omitempty"`
	Target       map[string]interface{} `json:"target,omitempty"`
}

type generateResponse struct {
	Data struct {
		TaskID     string `json:"taskId"`
		SauceJobID string `json:"sauceJobId"`
	} `json:"data"`
}

// GenerateTestCase submits a new test case authoring request. It always
// creates a brand new test case -- the Test Authoring API has no endpoint to
// re-author an existing one in place.
func (s *AuthoringService) GenerateTestCase(ctx context.Context, gr authoring.GenerateRequest) (authoring.GenerateTask, error) {
	body := generateRequestBody{
		Name:        gr.Name,
		TestSuiteID: gr.TestSuiteID,
		Tags:        gr.Tags,
		RunSettings: runSettingsBody{
			TestURL:      gr.RunSettings.TestURL,
			SCTunnelName: gr.RunSettings.SCTunnelName,
			Target:       gr.RunSettings.Target,
		},
		PromptSettings: gr.PromptSettings,
		Timeout:        gr.TimeoutMS,
	}

	req, err := s.newRequest(ctx, http.MethodPost, "/testcases/generate", body)
	if err != nil {
		return authoring.GenerateTask{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.GenerateTask{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return authoring.GenerateTask{}, authoringError(resp)
	}

	var gresp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&gresp); err != nil {
		return authoring.GenerateTask{}, err
	}

	return authoring.GenerateTask{
		TaskID:     gresp.Data.TaskID,
		SauceJobID: gresp.Data.SauceJobID,
		Status:     authoring.TaskPending,
	}, nil
}

type generateTaskResponse struct {
	Data struct {
		Status     string `json:"status"`
		TestCaseID string `json:"testCaseId"`
		Error      struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
		} `json:"error"`
	} `json:"data"`
}

// GetGenerateTask polls the status of a previously submitted generate task.
func (s *AuthoringService) GetGenerateTask(ctx context.Context, taskID string) (authoring.GenerateTask, error) {
	req, err := s.newRequest(ctx, http.MethodGet, fmt.Sprintf("/testcases/generate/%s", taskID), nil)
	if err != nil {
		return authoring.GenerateTask{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.GenerateTask{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return authoring.GenerateTask{}, authoring.ErrTestCaseNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return authoring.GenerateTask{}, authoringError(resp)
	}

	var tresp generateTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&tresp); err != nil {
		return authoring.GenerateTask{}, err
	}

	return authoring.GenerateTask{
		TaskID:      taskID,
		Status:      authoring.GenerateTaskStatus(tresp.Data.Status),
		TestCaseID:  tresp.Data.TestCaseID,
		ErrorCode:   tresp.Data.Error.Code,
		ErrorDetail: tresp.Data.Error.Detail,
	}, nil
}

type testCaseResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TestSuiteID string    `json:"testSuiteId"`
	Tags        []string  `json:"tags"`
	Creation    time.Time `json:"creationDate"`
	LastUpdate  time.Time `json:"lastUpdateDate"`
}

func toTestCase(tc testCaseResponse) authoring.TestCase {
	return authoring.TestCase{
		ID:           tc.ID,
		Name:         tc.Name,
		TestSuiteID:  tc.TestSuiteID,
		Tags:         tc.Tags,
		CreationDate: tc.Creation,
		LastUpdate:   tc.LastUpdate,
	}
}

type testCaseEnvelope struct {
	Data testCaseResponse `json:"data"`
}

// GetTestCase fetches a single test case by ID.
func (s *AuthoringService) GetTestCase(ctx context.Context, id string) (authoring.TestCase, error) {
	req, err := s.newRequest(ctx, http.MethodGet, fmt.Sprintf("/testcases/%s", id), nil)
	if err != nil {
		return authoring.TestCase{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.TestCase{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return authoring.TestCase{}, authoring.ErrTestCaseNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return authoring.TestCase{}, authoringError(resp)
	}

	var tc testCaseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&tc); err != nil {
		return authoring.TestCase{}, err
	}

	return toTestCase(tc.Data), nil
}

type testCaseListResponse struct {
	Data struct {
		Items []testCaseResponse `json:"items"`
		Total int                `json:"total"`
	} `json:"data"`
}

// ListTestCases lists test cases, optionally filtered by suite/search/tags,
// with skip/limit pagination (matching the pagination style already used by
// ListTestSuites and, notably, this repo's own storage.List).
func (s *AuthoringService) ListTestCases(ctx context.Context, opts authoring.ListTestCaseOptions) ([]authoring.TestCase, int, error) {
	uri, err := url.Parse(fmt.Sprintf("%s%s/testcases", s.URL, authoringBasePath))
	if err != nil {
		return nil, 0, err
	}

	query := uri.Query()
	if opts.TestSuiteID != "" {
		query.Set("testSuiteId", opts.TestSuiteID)
	}
	if opts.Search != "" {
		query.Set("search", opts.Search)
	}
	for _, t := range opts.Tags {
		query.Add("tags", t)
	}
	if opts.Skip > 0 {
		query.Set("skip", strconv.Itoa(opts.Skip))
	}
	if opts.Limit > 0 {
		query.Set("limit", strconv.Itoa(opts.Limit))
	}
	uri.RawQuery = query.Encode()

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, authoringError(resp)
	}

	var lresp testCaseListResponse
	if err := json.NewDecoder(resp.Body).Decode(&lresp); err != nil {
		return nil, 0, err
	}

	cases := make([]authoring.TestCase, 0, len(lresp.Data.Items))
	for _, tc := range lresp.Data.Items {
		cases = append(cases, toTestCase(tc))
	}
	return cases, lresp.Data.Total, nil
}

type runTestCaseBody struct {
	BuildName    string                   `json:"buildName,omitempty"`
	SCTunnelName string                   `json:"scTunnelName,omitempty"`
	Targets      []map[string]interface{} `json:"targets,omitempty"`
}

type testCaseRunResponse struct {
	Data struct {
		ID         string `json:"id"`
		TestCaseID string `json:"testCaseId"`
		Build      string `json:"build"`
		Jobs       []struct {
			ID         string          `json:"id"`
			SauceJobID string          `json:"sauceJobId"`
			Target     json.RawMessage `json:"target"`
			Name       string          `json:"name"`
			URL        string          `json:"url"`
			IsRDC      bool            `json:"isRdc"`
			Success    bool            `json:"success"`
			Error      string          `json:"error"`
		} `json:"jobs"`
	} `json:"data"`
}

// targetToString renders a job's "target" field as a human-readable string,
// regardless of whether the live API sends it as a bare string or as an
// object (e.g. capabilities/device info) -- this is only used for display
// purposes, so it degrades gracefully rather than failing to decode.
func targetToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"name", "browserName", "deviceName", "platformName"} {
			if v, ok := obj[key].(string); ok && v != "" {
				return v
			}
		}
	}

	return string(raw)
}

// RunTestCase triggers a run of a single test case and returns the job(s) it
// queued, with real job identifiers -- unlike RunTestSuite, no Builds API
// bridge is needed to find out what got queued.
func (s *AuthoringService) RunTestCase(ctx context.Context, id string, opts authoring.RunTestCaseOptions) (authoring.TestCaseRun, error) {
	body := runTestCaseBody{
		BuildName:    opts.BuildName,
		SCTunnelName: opts.SCTunnelName,
		Targets:      opts.Targets,
	}

	req, err := s.newRequest(ctx, http.MethodPost, fmt.Sprintf("/testcases/%s/run", id), body)
	if err != nil {
		return authoring.TestCaseRun{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.TestCaseRun{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return authoring.TestCaseRun{}, authoring.ErrTestCaseNotFound
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return authoring.TestCaseRun{}, authoringError(resp)
	}

	var rresp testCaseRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&rresp); err != nil {
		return authoring.TestCaseRun{}, err
	}

	jobs := make([]authoring.TestCaseJob, 0, len(rresp.Data.Jobs))
	for _, j := range rresp.Data.Jobs {
		jobs = append(jobs, authoring.TestCaseJob{
			ID:         j.ID,
			SauceJobID: j.SauceJobID,
			Target:     targetToString(j.Target),
			Name:       j.Name,
			URL:        j.URL,
			IsRDC:      j.IsRDC,
			Success:    j.Success,
			Error:      j.Error,
		})
	}

	return authoring.TestCaseRun{
		ID:         rresp.Data.ID,
		TestCaseID: rresp.Data.TestCaseID,
		BuildName:  rresp.Data.Build,
		Jobs:       jobs,
	}, nil
}

// DeleteTestCase deletes a test case by ID. Deleting a test case that
// doesn't exist is treated as a no-op, since the end state (no such test
// case) is what was asked for -- this matters for idempotent retry of
// reconciliation logic.
func (s *AuthoringService) DeleteTestCase(ctx context.Context, id string) error {
	req, err := s.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/testcases/%s", id), nil)
	if err != nil {
		return err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		return authoringError(resp)
	}
}

type testSuiteResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Tags          []string  `json:"tags"`
	TestCaseCount int       `json:"testCaseCount"`
	Creation      time.Time `json:"creationDate"`
	LastUpdate    time.Time `json:"lastUpdate"`
}

type testSuiteListResponse struct {
	Data struct {
		Items []testSuiteResponse `json:"items"`
		Total int                 `json:"total"`
	} `json:"data"`
}

type testSuiteEnvelope struct {
	Data testSuiteResponse `json:"data"`
}

func toTestSuite(r testSuiteResponse) authoring.TestSuite {
	return authoring.TestSuite{
		ID:            r.ID,
		Name:          r.Name,
		Tags:          r.Tags,
		TestCaseCount: r.TestCaseCount,
		CreationDate:  r.Creation,
		LastUpdate:    r.LastUpdate,
	}
}

// ListTestSuites lists test suites, optionally filtered by search term/tags.
func (s *AuthoringService) ListTestSuites(ctx context.Context, opts authoring.ListTestSuiteOptions) ([]authoring.TestSuite, error) {
	uri, err := url.Parse(fmt.Sprintf("%s%s/testsuites", s.URL, authoringBasePath))
	if err != nil {
		return nil, err
	}

	query := uri.Query()
	if opts.Search != "" {
		query.Set("search", opts.Search)
	}
	for _, t := range opts.Tags {
		query.Add("tags", t)
	}
	uri.RawQuery = query.Encode()

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.Username, s.AccessKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, authoringError(resp)
	}

	var lresp testSuiteListResponse
	if err := json.NewDecoder(resp.Body).Decode(&lresp); err != nil {
		return nil, err
	}

	suites := make([]authoring.TestSuite, 0, len(lresp.Data.Items))
	for _, r := range lresp.Data.Items {
		suites = append(suites, toTestSuite(r))
	}
	return suites, nil
}

// GetTestSuite fetches a single test suite by ID.
func (s *AuthoringService) GetTestSuite(ctx context.Context, id string) (authoring.TestSuite, error) {
	req, err := s.newRequest(ctx, http.MethodGet, fmt.Sprintf("/testsuites/%s", id), nil)
	if err != nil {
		return authoring.TestSuite{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.TestSuite{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return authoring.TestSuite{}, authoring.ErrTestSuiteNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return authoring.TestSuite{}, authoringError(resp)
	}

	var r testSuiteEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return authoring.TestSuite{}, err
	}
	return toTestSuite(r.Data), nil
}

type createTestSuiteBody struct {
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

// CreateTestSuite creates a new, empty test suite.
func (s *AuthoringService) CreateTestSuite(ctx context.Context, name string, tags []string) (authoring.TestSuite, error) {
	req, err := s.newRequest(ctx, http.MethodPost, "/testsuites", createTestSuiteBody{Name: name, Tags: tags})
	if err != nil {
		return authoring.TestSuite{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.TestSuite{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return authoring.TestSuite{}, authoringError(resp)
	}

	var r testSuiteEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return authoring.TestSuite{}, err
	}
	return toTestSuite(r.Data), nil
}

type updateTestSuiteBody struct {
	Name            string   `json:"name,omitempty"`
	AddTestCases    []string `json:"addTestCases,omitempty"`
	RemoveTestCases []string `json:"removeTestCases,omitempty"`
}

// UpdateTestSuite updates a test suite's metadata and/or membership.
func (s *AuthoringService) UpdateTestSuite(ctx context.Context, id string, opts authoring.UpdateTestSuiteOptions) (authoring.TestSuite, error) {
	body := updateTestSuiteBody{
		Name:            opts.Name,
		AddTestCases:    opts.AddTestCases,
		RemoveTestCases: opts.RemoveTestCases,
	}

	req, err := s.newRequest(ctx, http.MethodPost, fmt.Sprintf("/testsuites/%s", id), body)
	if err != nil {
		return authoring.TestSuite{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.TestSuite{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return authoring.TestSuite{}, authoring.ErrTestSuiteNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return authoring.TestSuite{}, authoringError(resp)
	}

	var r testSuiteEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return authoring.TestSuite{}, err
	}
	return toTestSuite(r.Data), nil
}

// DeleteTestSuite deletes a test suite by ID.
func (s *AuthoringService) DeleteTestSuite(ctx context.Context, id string) error {
	req, err := s.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/testsuites/%s", id), nil)
	if err != nil {
		return err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		return authoringError(resp)
	}
}

type runTestSuiteBody struct {
	BuildName string `json:"buildName,omitempty"`
}

type suiteRunResponse struct {
	Data struct {
		ID        string `json:"id"`
		OrgID     string `json:"orgId"`
		TeamID    string `json:"teamId"`
		UserID    string `json:"userId"`
		RunCount  int    `json:"runCount"`
		BuildName string `json:"buildName"`
	} `json:"data"`
}

// RunTestSuite triggers a run of every test case currently in the suite.
// Note: the response only reports how many jobs were queued (RunCount), not
// their individual test case/job IDs -- BuildName is the only correlation
// key available for finding the resulting jobs afterwards, so callers should
// always pass a unique, traceable buildName (e.g. including a commit SHA or
// PR number).
func (s *AuthoringService) RunTestSuite(ctx context.Context, id string, buildName string) (authoring.SuiteRun, error) {
	req, err := s.newRequest(ctx, http.MethodPost, fmt.Sprintf("/testsuites/%s/run", id), runTestSuiteBody{BuildName: buildName})
	if err != nil {
		return authoring.SuiteRun{}, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return authoring.SuiteRun{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return authoring.SuiteRun{}, authoring.ErrTestSuiteNotFound
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return authoring.SuiteRun{}, authoringError(resp)
	}

	var rresp suiteRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&rresp); err != nil {
		return authoring.SuiteRun{}, err
	}

	return authoring.SuiteRun{
		ID:        rresp.Data.ID,
		OrgID:     rresp.Data.OrgID,
		TeamID:    rresp.Data.TeamID,
		UserID:    rresp.Data.UserID,
		RunCount:  rresp.Data.RunCount,
		BuildName: rresp.Data.BuildName,
	}, nil
}
