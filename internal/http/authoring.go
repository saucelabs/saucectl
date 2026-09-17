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
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/region"
)

// authoringBasePath is the AI Authoring API's prefix on the region's API host.
// The entitlement check deliberately lives outside it (see IsAIAuthoringEnabled).
const authoringBasePath = "/ai-authoring/v1"

// aiAuthoringEntitlement is the entitlement name that gates the feature.
const aiAuthoringEntitlement = "ai_authoring.enabled"

// AuthoringService is the HTTP implementation of the authoring service
// interfaces (authoring.TestCaseService, TestSuiteService, ScheduleService,
// VariableService, ArtifactService and EntitlementReader).
//
// Authentication is HTTP Basic with username and access key. The published
// specification declares bearer JWT auth on every operation, but Basic auth is
// what the live service accepts and is the platform-wide convention (research
// R-001). It is applied in exactly one place, newRequest, so a future change
// is a one-line edit.
type AuthoringService struct {
	// Client is used for reads. It retries transport errors and 5xx responses
	// but, unlike the shared client, never retries 404: this API answers 404
	// as an ordinary outcome (TEST_CASE_NOT_FOUND, VARIABLE_NOT_FOUND, ...),
	// and retrying it would turn every miss into four requests and several
	// seconds (research R-010).
	Client *retryablehttp.Client
	// OnceClient is used for mutations and never retries. A retry after a
	// POST that succeeded server-side but failed to reach us would start a
	// second run or authoring task, consuming a second VM (research Open-5);
	// a retried PATCH or DELETE that carries a concurrency token would report
	// a spurious conflict for a change that actually went through.
	OnceClient *retryablehttp.Client
	// URL is the region's API base URL, e.g. https://api.us-west-1.saucelabs.com.
	URL       string
	Username  string
	AccessKey string
}

// NewAuthoringService creates a client for the given region and credentials.
// timeout bounds each individual request.
func NewAuthoringService(r region.Region, creds iam.Credentials, timeout time.Duration) AuthoringService {
	return AuthoringService{
		Client:     newAuthoringHTTPClient(timeout),
		OnceClient: newAuthoringOnceClient(timeout),
		URL:        r.APIBaseURL(),
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}
}

// newAuthoringHTTPClient is the shared retryable client with its 404 retry
// removed. It keeps the default policy's handling of transport errors and 5xx.
func newAuthoringHTTPClient(timeout time.Duration) *retryablehttp.Client {
	c := NewRetryableClient(timeout)
	c.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}
	return c
}

// newAuthoringOnceClient performs exactly one attempt per request.
func newAuthoringOnceClient(timeout time.Duration) *retryablehttp.Client {
	c := newAuthoringHTTPClient(timeout)
	c.RetryMax = 0
	return c
}

// envelope is the {"data": ...} wrapper around every successful body.
type envelope struct {
	Data json.RawMessage `json:"data"`
}

// errorEnvelope is the {"error": {...}} wrapper around every failure body.
type errorEnvelope struct {
	Error authoring.APIError `json:"error"`
}

// newRequest builds an authenticated request against the authoring API.
// query may be nil. body, when non-nil, is JSON-encoded.
func (c *AuthoringService) newRequest(ctx context.Context, method, path string, query url.Values, body any) (*retryablehttp.Request, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
	}

	u := c.URL + authoringBasePath + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var rdr io.Reader
	if payload != nil {
		rdr = bytes.NewReader(payload)
	}
	req, err := NewRetryableRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Username, c.AccessKey)
	return req, nil
}

// do sends the request with the given client and returns the response, or a
// decoded *authoring.APIError for any 4xx/5xx status. The caller must close the
// body of a returned response.
func (c *AuthoringService) do(client *retryablehttp.Client, req *retryablehttp.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		return nil, newAuthoringError(resp)
	}
	return resp, nil
}

// doJSON sends the request and decodes the "data" member of the response into
// out. out may be nil for endpoints that return 204.
func (c *AuthoringService) doJSON(client *retryablehttp.Client, req *retryablehttp.Request, out any) error {
	resp, err := c.do(client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	if len(env.Data) == 0 {
		return fmt.Errorf("decoding response: missing data envelope")
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decoding response data: %w", err)
	}
	return nil
}

// newAuthoringError decodes the error envelope of a failed response. It always
// records the HTTP status; when the body is not the documented envelope (a
// proxy error page, say) the raw text becomes the detail so nothing is lost.
func newAuthoringError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Code != "" {
		apiErr := env.Error
		apiErr.HTTPStatus = resp.StatusCode
		return &apiErr
	}

	detail := strings.TrimSpace(string(body))
	if detail == "" {
		detail = http.StatusText(resp.StatusCode)
	}
	return &authoring.APIError{HTTPStatus: resp.StatusCode, Detail: detail}
}

// listQuery encodes shared pagination. skip is omitted at zero; limit is sent
// whenever the caller set it, even when zero, because limit=0 is the
// service's count-only mode (research: supporting observations).
func listQuery(opts authoring.ListOptions) url.Values {
	q := url.Values{}
	if opts.Skip > 0 {
		q.Set("skip", strconv.Itoa(opts.Skip))
	}
	if opts.Limit != nil {
		q.Set("limit", strconv.Itoa(*opts.Limit))
	}
	return q
}

// setNonEmpty adds a single-valued parameter when it has a value.
func setNonEmpty(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

// addAll adds a repeatable parameter once per value. The service accepts a
// repeated key as an array.
func addAll(q url.Values, key string, values []string) {
	for _, v := range values {
		if v != "" {
			q.Add(key, v)
		}
	}
}

// ---- Test cases -----------------------------------------------------------

// ListTestCases implements authoring.TestCaseService.
func (c *AuthoringService) ListTestCases(ctx context.Context, opts authoring.ListTestCasesOptions) (authoring.List[authoring.TestCase], error) {
	q := listQuery(opts.ListOptions)
	setNonEmpty(q, "search", opts.Search)
	setNonEmpty(q, "startDate", opts.StartDate)
	setNonEmpty(q, "endDate", opts.EndDate)
	setNonEmpty(q, "userId", opts.UserID)
	setNonEmpty(q, "teamId", opts.TeamID)
	addAll(q, "testSuiteId", opts.TestSuiteIDs)
	addAll(q, "tags", opts.Tags)

	var out authoring.List[authoring.TestCase]
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases", q, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// GetTestCase implements authoring.TestCaseService.
func (c *AuthoringService) GetTestCase(ctx context.Context, id string) (authoring.TestCase, error) {
	var out authoring.TestCase
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// DeleteTestCase implements authoring.TestCaseService.
func (c *AuthoringService) DeleteTestCase(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/testcases/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return err
	}
	return c.doJSON(c.OnceClient, req, nil)
}

// RenameTestCase implements authoring.TestCaseService.
func (c *AuthoringService) RenameTestCase(ctx context.Context, id, name string) (authoring.TestCase, error) {
	var out authoring.TestCase
	body := struct {
		Name string `json:"name"`
	}{Name: name}
	req, err := c.newRequest(ctx, http.MethodPost, "/testcases/"+url.PathEscape(id)+"/rename", nil, body)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// RunTestCase implements authoring.TestCaseService. The revision-scoped path
// is described by the service but not declared in its specification; it is
// only used when revisionID is given.
func (c *AuthoringService) RunTestCase(ctx context.Context, id, revisionID string, opts authoring.RunOptions) (authoring.Run, error) {
	var out authoring.Run
	path := "/testcases/" + url.PathEscape(id) + "/run"
	if revisionID != "" {
		path += "/" + url.PathEscape(revisionID)
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// ListRuns implements authoring.TestCaseService. testCaseID is sent as the
// testCaseId query parameter, which is the filter the service actually
// honours: the path parameter alone returns every run in the organisation
// (research R-004, SC-002).
func (c *AuthoringService) ListRuns(ctx context.Context, testCaseID string, opts authoring.ListRunsOptions) (authoring.List[authoring.Run], error) {
	q := listQuery(opts.ListOptions)
	q.Set("testCaseId", testCaseID)
	setNonEmpty(q, "startDate", opts.StartDate)
	setNonEmpty(q, "endDate", opts.EndDate)
	setNonEmpty(q, "userId", opts.UserID)
	setNonEmpty(q, "teamId", opts.TeamID)

	var out authoring.List[authoring.Run]
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases/"+url.PathEscape(testCaseID)+"/runs", q, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// GetRun implements authoring.TestCaseService.
func (c *AuthoringService) GetRun(ctx context.Context, testCaseID, runID string) (authoring.Run, error) {
	var out authoring.Run
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases/"+url.PathEscape(testCaseID)+"/runs/"+url.PathEscape(runID), nil, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// ListTags implements authoring.TestCaseService.
func (c *AuthoringService) ListTags(ctx context.Context) ([]string, error) {
	var out []string
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases/tags", nil, nil)
	if err != nil {
		return nil, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// Generate implements authoring.TestCaseService.
func (c *AuthoringService) Generate(ctx context.Context, opts authoring.GenerateOptions) (authoring.GenerateTask, error) {
	var out authoring.GenerateTask
	req, err := c.newRequest(ctx, http.MethodPost, "/testcases/generate", nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// GenerationStatus implements authoring.TestCaseService.
func (c *AuthoringService) GenerationStatus(ctx context.Context, taskID string) (authoring.GenerationState, error) {
	var out authoring.GenerationState
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases/generate/"+url.PathEscape(taskID), nil, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// Code implements authoring.TestCaseService.
func (c *AuthoringService) Code(ctx context.Context, id, target string) (string, error) {
	var out struct {
		Code string `json:"code"`
	}
	q := url.Values{}
	q.Set("target", target)
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases/"+url.PathEscape(id)+"/code", q, nil)
	if err != nil {
		return "", err
	}
	return out.Code, c.doJSON(c.Client, req, &out)
}

// CodeTargets implements authoring.TestCaseService.
func (c *AuthoringService) CodeTargets(ctx context.Context, id string) ([]string, error) {
	var out struct {
		Targets []string `json:"targets"`
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/testcases/"+url.PathEscape(id)+"/code/targets", nil, nil)
	if err != nil {
		return nil, err
	}
	return out.Targets, c.doJSON(c.Client, req, &out)
}

// ---- Test suites ----------------------------------------------------------

// ListTestSuites implements authoring.TestSuiteService.
func (c *AuthoringService) ListTestSuites(ctx context.Context, opts authoring.ListTestSuitesOptions) (authoring.List[authoring.TestSuite], error) {
	q := listQuery(opts.ListOptions)
	addAll(q, "ids", opts.IDs)
	setNonEmpty(q, "search", opts.Search)
	setNonEmpty(q, "startDate", opts.StartDate)
	setNonEmpty(q, "endDate", opts.EndDate)
	setNonEmpty(q, "userId", opts.UserID)
	setNonEmpty(q, "teamId", opts.TeamID)

	var out authoring.List[authoring.TestSuite]
	req, err := c.newRequest(ctx, http.MethodGet, "/testsuites", q, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// GetTestSuite implements authoring.TestSuiteService.
func (c *AuthoringService) GetTestSuite(ctx context.Context, id string) (authoring.TestSuite, error) {
	var out authoring.TestSuite
	req, err := c.newRequest(ctx, http.MethodGet, "/testsuites/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// CreateTestSuite implements authoring.TestSuiteService.
func (c *AuthoringService) CreateTestSuite(ctx context.Context, opts authoring.CreateTestSuiteOptions) (authoring.TestSuite, error) {
	var out authoring.TestSuite
	req, err := c.newRequest(ctx, http.MethodPost, "/testsuites", nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// UpdateTestSuite implements authoring.TestSuiteService.
func (c *AuthoringService) UpdateTestSuite(ctx context.Context, id string, opts authoring.UpdateTestSuiteOptions) (authoring.TestSuite, error) {
	var out authoring.TestSuite
	req, err := c.newRequest(ctx, http.MethodPost, "/testsuites/"+url.PathEscape(id), nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// DeleteTestSuite implements authoring.TestSuiteService.
func (c *AuthoringService) DeleteTestSuite(ctx context.Context, id string, deleteTestCases bool) error {
	body := struct {
		DeleteTestCases bool `json:"deleteTestCases"`
	}{DeleteTestCases: deleteTestCases}
	req, err := c.newRequest(ctx, http.MethodDelete, "/testsuites/"+url.PathEscape(id), nil, body)
	if err != nil {
		return err
	}
	return c.doJSON(c.OnceClient, req, nil)
}

// RunTestSuite implements authoring.TestSuiteService.
func (c *AuthoringService) RunTestSuite(ctx context.Context, id, buildName string) (authoring.SuiteRun, error) {
	var out authoring.SuiteRun
	// Omitted when unset, matching RunOptions on the test-case run endpoint.
	// Sending an explicit null here was never asked for by the API and never
	// verified, unlike scTunnelName, where a null is load-bearing.
	body := struct {
		BuildName string `json:"buildName,omitempty"`
	}{BuildName: buildName}
	req, err := c.newRequest(ctx, http.MethodPost, "/testsuites/"+url.PathEscape(id)+"/run", nil, body)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// ---- Schedules ------------------------------------------------------------

// ListSchedules implements authoring.ScheduleService.
func (c *AuthoringService) ListSchedules(ctx context.Context, opts authoring.ListSchedulesOptions) (authoring.List[authoring.TestSchedule], error) {
	q := listQuery(opts.ListOptions)
	addAll(q, "ids", opts.IDs)
	setNonEmpty(q, "search", opts.Search)
	setNonEmpty(q, "startDate", opts.StartDate)
	setNonEmpty(q, "endDate", opts.EndDate)
	setNonEmpty(q, "userId", opts.UserID)
	setNonEmpty(q, "teamId", opts.TeamID)
	addAll(q, "testSuiteIds", opts.TestSuiteIDs)

	var out authoring.List[authoring.TestSchedule]
	req, err := c.newRequest(ctx, http.MethodGet, "/test-schedules", q, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// GetSchedule implements authoring.ScheduleService.
func (c *AuthoringService) GetSchedule(ctx context.Context, id string) (authoring.TestSchedule, error) {
	var out authoring.TestSchedule
	req, err := c.newRequest(ctx, http.MethodGet, "/test-schedules/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// CreateSchedule implements authoring.ScheduleService.
func (c *AuthoringService) CreateSchedule(ctx context.Context, opts authoring.CreateScheduleOptions) (authoring.TestSchedule, error) {
	var out authoring.TestSchedule
	req, err := c.newRequest(ctx, http.MethodPost, "/test-schedules", nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// UpdateSchedule implements authoring.ScheduleService.
func (c *AuthoringService) UpdateSchedule(ctx context.Context, id string, opts authoring.UpdateScheduleOptions) (authoring.TestSchedule, error) {
	var out authoring.TestSchedule
	req, err := c.newRequest(ctx, http.MethodPost, "/test-schedules/"+url.PathEscape(id), nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// DeleteSchedule implements authoring.ScheduleService.
func (c *AuthoringService) DeleteSchedule(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/test-schedules/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return err
	}
	return c.doJSON(c.OnceClient, req, nil)
}

// ---- Variables ------------------------------------------------------------

// ListVariables implements authoring.VariableService.
func (c *AuthoringService) ListVariables(ctx context.Context, opts authoring.ListVariablesOptions) (authoring.List[authoring.Variable], error) {
	q := listQuery(opts.ListOptions)
	setNonEmpty(q, "scope", string(opts.Scope))
	setNonEmpty(q, "testSuiteId", opts.TestSuiteID)
	setNonEmpty(q, "testCaseId", opts.TestCaseID)
	setNonEmpty(q, "search", opts.Search)

	var out authoring.List[authoring.Variable]
	req, err := c.newRequest(ctx, http.MethodGet, "/variables", q, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// GetVariable implements authoring.VariableService.
func (c *AuthoringService) GetVariable(ctx context.Context, id string) (authoring.Variable, error) {
	var out authoring.Variable
	req, err := c.newRequest(ctx, http.MethodGet, "/variables/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.Client, req, &out)
}

// CreateVariable implements authoring.VariableService.
func (c *AuthoringService) CreateVariable(ctx context.Context, opts authoring.CreateVariableOptions) (authoring.Variable, error) {
	var out authoring.Variable
	req, err := c.newRequest(ctx, http.MethodPost, "/variables", nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// UpdateVariable implements authoring.VariableService. The concurrency token
// travels in the body here — but in the query string on delete.
func (c *AuthoringService) UpdateVariable(ctx context.Context, id string, opts authoring.UpdateVariableOptions) (authoring.Variable, error) {
	var out authoring.Variable
	req, err := c.newRequest(ctx, http.MethodPatch, "/variables/"+url.PathEscape(id), nil, opts)
	if err != nil {
		return out, err
	}
	return out, c.doJSON(c.OnceClient, req, &out)
}

// DeleteVariable implements authoring.VariableService. The concurrency token
// travels in the query string here — but in the body on update.
func (c *AuthoringService) DeleteVariable(ctx context.Context, id, expectedLastUpdate string) error {
	q := url.Values{}
	q.Set("expectedLastUpdate", expectedLastUpdate)
	req, err := c.newRequest(ctx, http.MethodDelete, "/variables/"+url.PathEscape(id), q, nil)
	if err != nil {
		return err
	}
	return c.doJSON(c.OnceClient, req, nil)
}

// ---- Artifacts ------------------------------------------------------------

// DownloadArtifact implements authoring.ArtifactService. The response carries
// no Content-Type, so nothing about the file's kind can be inferred here.
func (c *AuthoringService) DownloadArtifact(ctx context.Context, id string) (io.ReadCloser, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/storage/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Del("Accept")
	resp, err := c.do(c.Client, req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ---- Entitlement ----------------------------------------------------------

// IsAIAuthoringEnabled implements authoring.EntitlementReader against the
// platform entitlements API on the same host — a different API from the
// authoring service, absent from its specification (research R-009):
//
//	GET {URL}/v2/entitlements/entities/org/{orgID}?entitlements=ai_authoring.enabled
//	200 {"level":"org","uuid":"…","entitlements":[{"name":"ai_authoring.enabled","value":true,"region":"GLOBAL"}]}
//
// value was observed as a JSON boolean but is undocumented, so it is decoded
// permissively: true, "true" and 1 all count as enabled. A missing
// entitlement entry counts as disabled. Any non-200 response is an error, so
// the caller can distinguish "not in your plan" from "could not verify".
func (c *AuthoringService) IsAIAuthoringEnabled(ctx context.Context, orgID string) (bool, error) {
	u := fmt.Sprintf("%s/v2/entitlements/entities/org/%s?entitlements=%s", c.URL, url.PathEscape(orgID), aiAuthoringEntitlement)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return false, fmt.Errorf("entitlement check failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Entitlements []struct {
			Name  string `json:"name"`
			Value any    `json:"value"`
		} `json:"entitlements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("decoding entitlement response: %w", err)
	}

	for _, e := range out.Entitlements {
		if e.Name == aiAuthoringEntitlement {
			return entitlementEnabled(e.Value), nil
		}
	}
	return false, nil
}

// entitlementEnabled interprets the undocumented entitlement value leniently.
func entitlementEnabled(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	case float64:
		return t == 1
	default:
		return false
	}
}
