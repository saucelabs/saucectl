package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/region"
)

// newTestAuthoringService points a client at the test server with retries
// made fast, so tests that count attempts finish in milliseconds.
func newTestAuthoringService(srv *httptest.Server) AuthoringService {
	c := NewAuthoringService(region.USWest1, iam.Credentials{Username: "user", AccessKey: "key"}, 5*time.Second)
	c.URL = srv.URL
	c.Client.RetryWaitMin = 1 * time.Millisecond
	c.Client.RetryWaitMax = 1 * time.Millisecond
	c.OnceClient.RetryWaitMin = 1 * time.Millisecond
	c.OnceClient.RetryWaitMax = 1 * time.Millisecond
	return c
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func TestAuthoringService_BasicAuthAndEnvelope(t *testing.T) {
	var gotPath, gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		writeJSON(w, 200, `{"data":{"id":"6a882c1dc8b4482c166e96c9","name":"test","tags":[],"revisions":[],"runSettings":{"scTunnelName":"","primaryTarget":{"capabilities":{"browserName":"chrome"},"isRdc":false},"runTargets":[]}}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	tc, err := c.GetTestCase(context.Background(), "6a882c1dc8b4482c166e96c9")
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/ai-authoring/v1/testcases/6a882c1dc8b4482c166e96c9" {
		t.Errorf("path = %q", gotPath)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:key"))
	if gotAuth != want {
		t.Errorf("Authorization = %q, want basic auth (research R-001)", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q", gotAccept)
	}
	if tc.Name != "test" {
		t.Errorf("envelope not unwrapped: %+v", tc)
	}
	if tc.RunSettings.TunnelName == nil || *tc.RunSettings.TunnelName != "" {
		t.Error("empty stored tunnel name lost in decoding")
	}
}

func TestAuthoringService_ErrorEnvelopeWithUndocumentedData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 400, `{"error":{"code":"INVALID_QUERY","detail":"Invalid query string parameters.","data":[{"code":"custom","path":[],"message":"scope-specific id is required: scope=testSuite requires testSuiteId."}]}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	_, err := c.ListVariables(context.Background(), authoring.ListVariablesOptions{Scope: authoring.ScopeTestSuite})
	if !errors.Is(err, authoring.ErrInvalidQuery) {
		t.Fatalf("expected ErrInvalidQuery, got %v", err)
	}
	var apiErr *authoring.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.HTTPStatus != 400 {
		t.Errorf("HTTPStatus = %d", apiErr.HTTPStatus)
	}
	if !strings.Contains(err.Error(), "scope-specific id is required") {
		t.Errorf("the actionable message from error.data[] is missing: %s", err)
	}
}

func TestAuthoringService_NonEnvelopeErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(502)
		_, _ = io.WriteString(w, "<html>Bad Gateway</html>")
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	_, err := c.GetTestSuite(context.Background(), "x")
	var apiErr *authoring.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != 502 || !strings.Contains(apiErr.Detail, "Bad Gateway") {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestAuthoringService_NotFoundIsAnsweredInOneRequest(t *testing.T) {
	// The shared client retries 404 for APIs where it means propagation
	// delay. This API returns 404 as an ordinary answer, so exactly one
	// request must be made (research R-010, SC-007).
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeJSON(w, 404, `{"error":{"code":"TEST_CASE_NOT_FOUND","detail":"Test case not found."}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	_, err := c.GetTestCase(context.Background(), "000000000000000000000000")
	if !errors.Is(err, authoring.ErrTestCaseNotFound) {
		t.Fatalf("expected ErrTestCaseNotFound, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("handler invoked %d times, want exactly 1", n)
	}
}

func TestAuthoringService_ServerErrorRetriesReadsButNeverMutations(t *testing.T) {
	// Reads may be retried on 5xx. Mutations must not be: a retried run
	// start could consume a second VM (research Open-5).
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeJSON(w, 503, `{"error":{"code":"UNAVAILABLE","detail":"try later"}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)

	_, _ = c.ListTags(context.Background())
	if n := atomic.LoadInt32(&calls); n <= 1 {
		t.Errorf("read on 5xx made %d request(s), expected retries", n)
	}

	atomic.StoreInt32(&calls, 0)
	_, err := c.RunTestCase(context.Background(), "id", "", authoring.RunOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("mutation on 5xx made %d requests, want exactly 1", n)
	}
}

func TestAuthoringService_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	if err := c.DeleteTestCase(context.Background(), "abc"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthoringService_ListRunsSendsTestCaseIDQuery(t *testing.T) {
	// Without the query parameter the service returns every run in the
	// organisation (research R-004, SC-002).
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, 200, `{"data":{"items":[],"total":0}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	limit := 5
	_, err := c.ListRuns(context.Background(), "6a882c1dc8b4482c166e96c9", authoring.ListRunsOptions{ListOptions: authoring.ListOptions{Skip: 10, Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"testCaseId=6a882c1dc8b4482c166e96c9", "skip=10", "limit=5"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q lacks %q", gotQuery, want)
		}
	}
}

func TestAuthoringService_LimitIsSentWhenSetEvenIfZero(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		writeJSON(w, 200, `{"data":{"items":[],"total":187}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	zero := 0
	_, _ = c.ListTestCases(context.Background(), authoring.ListTestCasesOptions{ListOptions: authoring.ListOptions{Limit: &zero}})
	_, _ = c.ListTestCases(context.Background(), authoring.ListTestCasesOptions{Tags: []string{"Login", "login"}, TestSuiteIDs: []string{"a", "b"}})

	if queries[0] != "limit=0" {
		t.Errorf("count-only request sent %q, want limit=0", queries[0])
	}
	if strings.Contains(queries[1], "limit") {
		t.Errorf("unset limit must be omitted, got %q", queries[1])
	}
	for _, want := range []string{"tags=Login", "tags=login", "testSuiteId=a", "testSuiteId=b"} {
		if !strings.Contains(queries[1], want) {
			t.Errorf("query %q lacks repeated parameter %q", queries[1], want)
		}
	}
}

func TestAuthoringService_RunTestCaseBodyAndRevisionPath(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		writeJSON(w, 200, `{"data":{"id":"run","testCaseId":"tc","build":"b - 1","jobs":[{"id":"j","name":"n","sauceJobId":"s","target":{"capabilities":{},"isRdc":false}}]}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	run, err := c.RunTestCase(context.Background(), "tc", "rev1", authoring.RunOptions{BuildName: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/ai-authoring/v1/testcases/tc/run/rev1" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody != `{"buildName":"b","scTunnelName":null}` {
		t.Errorf("body = %s; an absent tunnel must be sent as explicit null (research Open-3)", gotBody)
	}
	if run.Done() {
		t.Error("start response has no success and must not read as done")
	}
	if run.Build != "b - 1" {
		t.Errorf("build = %q", run.Build)
	}
}

func TestAuthoringService_VariableConcurrencyToken(t *testing.T) {
	// The token goes in the body on update and in the query string on delete.
	var updateBody, deleteQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			b, _ := io.ReadAll(r.Body)
			updateBody = string(b)
			writeJSON(w, 200, `{"data":{"id":"v","scope":"org","name":"n","isSecret":true,"lastUpdate":"2026-09-05T15:00:00.123Z"}}`)
		case http.MethodDelete:
			deleteQuery = r.URL.RawQuery
			writeJSON(w, 412, `{"error":{"code":"VARIABLE_VERSION_CONFLICT","detail":"stale"}}`)
		}
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	secret := true
	v, err := c.UpdateVariable(context.Background(), "v", authoring.UpdateVariableOptions{IsSecret: &secret, ExpectedLastUpdate: "2026-09-05T14:59:59.000Z"})
	if err != nil {
		t.Fatal(err)
	}
	if updateBody != `{"isSecret":true,"expectedLastUpdate":"2026-09-05T14:59:59.000Z"}` {
		t.Errorf("update body = %s", updateBody)
	}
	if v.Value != "" {
		t.Errorf("secret value leaked: %q", v.Value)
	}

	err = c.DeleteVariable(context.Background(), "v", "2026-09-05T14:59:59.000Z")
	if !errors.Is(err, authoring.ErrVariableVersionConflict) {
		t.Fatalf("expected ErrVariableVersionConflict, got %v", err)
	}
	if deleteQuery != "expectedLastUpdate=2026-09-05T14%3A59%3A59.000Z" {
		t.Errorf("delete query = %q", deleteQuery)
	}
}

func TestAuthoringService_DeleteTestSuiteCascadeFlag(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	if err := c.DeleteTestSuite(context.Background(), "s", true); err != nil {
		t.Fatal(err)
	}
	if gotBody != `{"deleteTestCases":true}` {
		t.Errorf("body = %s", gotBody)
	}
}

func TestAuthoringService_CodeAndTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/code/targets"):
			writeJSON(w, 200, `{"data":{"targets":["typescript_playwright","python_selenium"]}}`)
		case strings.HasSuffix(r.URL.Path, "/code"):
			if r.URL.Query().Get("target") != "typescript_playwright" {
				writeJSON(w, 404, `{"error":{"code":"CODE_GENERATION_TARGET_NOT_FOUND"}}`)
				return
			}
			writeJSON(w, 200, `{"data":{"code":"import { test } from '@playwright/test';"}}`)
		}
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	targets, err := c.CodeTargets(context.Background(), "tc")
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets = %v, %v", targets, err)
	}
	code, err := c.Code(context.Background(), "tc", "typescript_playwright")
	if err != nil || !strings.HasPrefix(code, "import") {
		t.Fatalf("code = %q, %v", code, err)
	}
	_, err = c.Code(context.Background(), "tc", "cobol")
	if !errors.Is(err, authoring.ErrCodeGenerationTargetNotFound) {
		t.Errorf("expected ErrCodeGenerationTargetNotFound, got %v", err)
	}
}

func TestAuthoringService_GenerateAndStatus(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			writeJSON(w, 202, `{"data":{"taskId":"task","sauceJobId":"job"}}`)
			return
		}
		writeJSON(w, 200, `{"data":{"status":"IN_PROGRESS","steps":[{"action":{"type":"go_to_url","args":{"reasoning":"r","url":"https://x"}},"result":{"success":true}}],"reasoning":[]}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	task, err := c.Generate(context.Background(), authoring.GenerateOptions{
		Name:           "n",
		RunSettings:    authoring.GenerateRunSettings{Target: authoring.Target{Capabilities: map[string]any{"browserName": "chrome"}}},
		PromptSettings: authoring.PromptSettings{Intent: "do it"},
		TimeoutMillis:  120000,
	})
	if err != nil || task.TaskID != "task" {
		t.Fatalf("task = %+v, %v", task, err)
	}
	if !strings.Contains(gotBody, `"timeout":120000`) || strings.Contains(gotBody, `"isRdc"`) {
		t.Errorf("body = %s", gotBody)
	}

	state, err := c.GenerationStatus(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if state.Done() || len(state.Steps) != 1 || state.Steps[0].Action.Summary() != "go_to_url https://x" {
		t.Errorf("state = %+v", state)
	}
}

func TestAuthoringService_DownloadArtifact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/storage/missing") {
			writeJSON(w, 404, `{"error":{"code":"FILE_NOT_FOUND"}}`)
			return
		}
		// No Content-Type on purpose: the live service sends none.
		w.WriteHeader(200)
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	rc, err := c.DownloadArtifact(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "\x89PNG" {
		t.Errorf("body = %q", b)
	}
	_, err = c.DownloadArtifact(context.Background(), "missing")
	if !errors.Is(err, authoring.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestAuthoringService_IsAIAuthoringEnabled(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    bool
		wantErr bool
	}{
		{name: "boolean true", status: 200, body: `{"level":"org","entitlements":[{"name":"ai_authoring.enabled","value":true,"region":"GLOBAL"}]}`, want: true},
		{name: "string true", status: 200, body: `{"entitlements":[{"name":"ai_authoring.enabled","value":"true"}]}`, want: true},
		{name: "numeric one", status: 200, body: `{"entitlements":[{"name":"ai_authoring.enabled","value":1}]}`, want: true},
		{name: "false", status: 200, body: `{"entitlements":[{"name":"ai_authoring.enabled","value":false}]}`, want: false},
		{name: "missing entitlement", status: 200, body: `{"entitlements":[{"name":"other","value":true}]}`, want: false},
		{name: "unauthorized is an error, not a no", status: 401, body: `{"detail":"nope"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
				writeJSON(w, tt.status, tt.body)
			}))
			defer srv.Close()

			c := newTestAuthoringService(srv)
			c.Client.RetryMax = 0
			got, err := c.IsAIAuthoringEnabled(context.Background(), "org-1")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("enabled = %v, want %v", got, tt.want)
			}
			if gotPath != "/v2/entitlements/entities/org/org-1" || gotQuery != "entitlements=ai_authoring.enabled" {
				t.Errorf("request = %s?%s; the entitlement API is not under the authoring base path", gotPath, gotQuery)
			}
		})
	}
}

func TestAuthoringService_UpdateScheduleSendsNullToClear(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, 200, `{"data":{"id":"s","name":"n","settings":{"cron":"0 * * * *","timezone":"UTC","runningUserId":"u"},"state":{"stateName":"ENABLED"},"testSuiteIds":["a"]}}`)
	}))
	defer srv.Close()

	c := newTestAuthoringService(srv)
	empty := ""
	_, err := c.UpdateSchedule(context.Background(), "s", authoring.UpdateScheduleOptions{
		Settings: &authoring.ScheduleSettingsPatch{Cron: "0 * * * *", Timezone: "UTC", RunningUserID: "u", TunnelName: &empty},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings := gotBody["settings"].(map[string]any)
	if v, present := settings["scTunnelName"]; !present || v != nil {
		t.Errorf("scTunnelName = %v (present=%v), want explicit null", v, present)
	}
	if _, present := settings["buildName"]; present {
		t.Error("buildName must be omitted when not being changed")
	}
}
