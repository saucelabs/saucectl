package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/aiauthoring"
	"github.com/saucelabs/saucectl/internal/iam"
)

func newTestAIAuthoringService(url string) AIAuthoringService {
	c := newAIRetryableClient(10 * time.Second)
	c.RetryMax = 0

	return AIAuthoringService{
		Client:     c,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		URL:        url,
		Credentials: iam.Credentials{
			Username:  "user",
			AccessKey: "key",
		},
	}
}

func TestAIAuthoringService_ListTestCases(t *testing.T) {
	testCases := []struct {
		name      string
		opts      aiauthoring.ListOptions
		wantQuery string
		response  string
		want      aiauthoring.List
		wantErr   bool
	}{
		{
			name:     "bare array response",
			response: `[{"id":"tc1","name":"Cart"},{"id":"tc2","name":"Login"}]`,
			want: aiauthoring.List{
				Items: []aiauthoring.TestCase{
					{ID: "tc1", Name: "Cart"},
					{ID: "tc2", Name: "Login"},
				},
				Total: 2,
			},
		},
		{
			name:      "items envelope with total and query params",
			opts:      aiauthoring.ListOptions{Search: "cart", Limit: 10, Skip: 5, TestSuiteID: "suite1"},
			wantQuery: "limit=10&search=cart&skip=5&testSuiteId=suite1",
			response:  `{"items":[{"id":"tc1","name":"Cart"}],"total":42}`,
			want: aiauthoring.List{
				Items: []aiauthoring.TestCase{{ID: "tc1", Name: "Cart"}},
				Total: 42,
			},
		},
		{
			name:     "results nested under data with count",
			response: `{"data":{"results":[{"id":"tc1","name":"Cart"}],"count":7}}`,
			want: aiauthoring.List{
				Items: []aiauthoring.TestCase{{ID: "tc1", Name: "Cart"}},
				Total: 7,
			},
		},
		{
			name:     "empty object response",
			response: `{}`,
			want:     aiauthoring.List{},
		},
		{
			name:     "malformed response",
			response: `"nope"`,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/ai-authoring/testcases" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.URL.RawQuery != tc.wantQuery {
					t.Errorf("query = %q, want %q", r.URL.RawQuery, tc.wantQuery)
				}
				if user, pass, _ := r.BasicAuth(); user != "user" || pass != "key" {
					t.Errorf("unexpected basic auth: %s:%s", user, pass)
				}
				if rb := r.Header.Get("requested-by"); rb == "" {
					t.Error("requested-by header is not set")
				}
				fmt.Fprint(w, tc.response)
			}))
			defer ts.Close()

			c := newTestAIAuthoringService(ts.URL)
			got, err := c.ListTestCases(context.Background(), tc.opts)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %t", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestAIAuthoringService_GetTestCase(t *testing.T) {
	testCases := []struct {
		name     string
		id       string
		status   int
		response string
		want     aiauthoring.TestCase
		wantErr  string
	}{
		{
			name:     "plain response",
			id:       "tc1",
			status:   http.StatusOK,
			response: `{"id":"tc1","name":"Cart","status":"SAVED"}`,
			want:     aiauthoring.TestCase{ID: "tc1", Name: "Cart", Status: "SAVED"},
		},
		{
			name:     "data envelope",
			id:       "tc1",
			status:   http.StatusOK,
			response: `{"data":{"id":"tc1","name":"Cart","runSettings":{"testUrl":"https://example.com"}}}`,
			want: aiauthoring.TestCase{
				ID:          "tc1",
				Name:        "Cart",
				RunSettings: &aiauthoring.RunSettings{TestURL: "https://example.com"},
			},
		},
		{
			name:    "not found",
			id:      "nope",
			status:  http.StatusNotFound,
			wantErr: `test case "nope" not found`,
		},
		{
			name:     "not found with backend detail",
			id:       "draft",
			status:   http.StatusNotFound,
			response: `{"error":{"code":"TEST_CASE_REVISION_NOT_FOUND","detail":"Test case revision not found."}}`,
			wantErr:  `test case "draft": Test case revision not found. (TEST_CASE_REVISION_NOT_FOUND)`,
		},
		{
			name:    "unauthorized",
			id:      "tc1",
			status:  http.StatusUnauthorized,
			wantErr: "rejected the provided credentials",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if want := "/v1/ai-authoring/testcases/" + tc.id; r.URL.Path != want {
					t.Errorf("path = %s, want %s", r.URL.Path, want)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.response)
			}))
			defer ts.Close()

			c := newTestAIAuthoringService(ts.URL)
			got, err := c.GetTestCase(context.Background(), tc.id)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got none")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %q, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestAIAuthoringService_RenameTestCase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if want := "/v1/ai-authoring/testcases/tc1/rename"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if body.Name != "New Name" {
			t.Errorf("name = %q, want %q", body.Name, "New Name")
		}
		fmt.Fprintf(w, `{"id":"tc1","name":%q}`, body.Name)
	}))
	defer ts.Close()

	c := newTestAIAuthoringService(ts.URL)
	got, err := c.RenameTestCase(context.Background(), "tc1", "New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("name = %q, want %q", got.Name, "New Name")
	}
}

func TestAIAuthoringService_RunTestCase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if want := "/v1/ai-authoring/testcases/tc1/run"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		var req aiauthoring.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if req.BuildName != "my-build" || req.SCTunnelName != "my-tunnel" {
			t.Errorf("unexpected request: %+v", req)
		}
		if len(req.Targets) != 1 || req.Targets[0].Capabilities["browserName"] != "chrome" {
			t.Errorf("unexpected targets: %+v", req.Targets)
		}
		fmt.Fprint(w, `{"data":{"id":"run1","testCaseId":"tc1","build":"my-build",
			"jobs":[{"id":"job1","name":"Chrome","target":{"capabilities":{"browserName":"chrome"},"isRdc":false},"url":"https://app/tests/job1"}],
			"testUrl":"https://app/builds/b1"}}`)
	}))
	defer ts.Close()

	c := newTestAIAuthoringService(ts.URL)
	run, err := c.RunTestCase(context.Background(), "tc1", aiauthoring.RunRequest{
		BuildName:    "my-build",
		SCTunnelName: "my-tunnel",
		Targets: []aiauthoring.RunTarget{
			{Capabilities: aiauthoring.RunCapabilities{"browserName": "chrome"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID != "run1" || len(run.Jobs) != 1 || run.Jobs[0].ID != "job1" {
		t.Errorf("unexpected run: %+v", run)
	}
	if run.Jobs[0].Target.IsRDC {
		t.Error("job must not be flagged as RDC")
	}
}

func TestAIAuthoringService_GetTestSuite(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/v1/ai-authoring/testsuites/suite1"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		fmt.Fprint(w, `{"data":{"id":"suite1","name":"Example Test Suite","testCaseCount":3,"runCount":1}}`)
	}))
	defer ts.Close()

	c := newTestAIAuthoringService(ts.URL)
	suite, err := c.GetTestSuite(context.Background(), "suite1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suite.Name != "Example Test Suite" || suite.TestCaseCount != 3 {
		t.Errorf("unexpected suite: %+v", suite)
	}
}

func TestAIAuthoringService_GetTestCaseRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/v1/ai-authoring/testcases/tc1/runs/run1"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		fmt.Fprint(w, `{"data":{"id":"run1","testCaseId":"tc1",
			"jobs":[{"id":"internal1","sauceJobId":"57c325ee","name":"Pixel 7",
			"url":"https://app.saucelabs.com/tests/57c325ee","success":true,
			"target":{"capabilities":{"platformName":"Android"},"isRdc":true}}]}}`)
	}))
	defer ts.Close()

	c := newTestAIAuthoringService(ts.URL)
	run, err := c.GetTestCaseRun(context.Background(), "tc1", "run1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(run.Jobs))
	}
	j := run.Jobs[0]
	if j.SauceJobID != "57c325ee" || !j.Done() || !j.Passed() || !j.Target.IsRDC {
		t.Errorf("unexpected job: %+v", j)
	}
}

func TestAIAuthoringService_DeleteTestCase(t *testing.T) {
	testCases := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "deleted", status: http.StatusNoContent},
		{name: "not found", status: http.StatusNotFound, wantErr: true},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				w.WriteHeader(tc.status)
			}))
			defer ts.Close()

			c := newTestAIAuthoringService(ts.URL)
			err := c.DeleteTestCase(context.Background(), "tc1")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %t", err, tc.wantErr)
			}
			if calls != 1 {
				t.Errorf("delete must never be retried, got %d calls", calls)
			}
		})
	}
}

func TestAIAuthoringService_IsAIAuthoringEnabled(t *testing.T) {
	testCases := []struct {
		name     string
		orgID    string
		response string
		want     bool
		wantErr  bool
	}{
		{
			name:     "enabled as bool",
			orgID:    "org1",
			response: `{"entitlements":[{"name":"ai_authoring.enabled","value":true}]}`,
			want:     true,
		},
		{
			name:     "enabled as string",
			orgID:    "org1",
			response: `{"entitlements":[{"name":"ai_authoring.enabled","value":"true"}]}`,
			want:     true,
		},
		{
			name:     "disabled",
			orgID:    "org1",
			response: `{"entitlements":[{"name":"ai_authoring.enabled","value":false}]}`,
			want:     false,
		},
		{
			name:     "missing entitlement fails closed",
			orgID:    "org1",
			response: `{"entitlements":[]}`,
			want:     false,
		},
		{
			name:    "missing org id",
			orgID:   "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if want := "/v2/entitlements/entities/org/" + tc.orgID; r.URL.Path != want {
					t.Errorf("path = %s, want %s", r.URL.Path, want)
				}
				if got := r.URL.Query().Get("entitlements"); got != "ai_authoring.enabled" {
					t.Errorf("entitlements param = %q", got)
				}
				fmt.Fprint(w, tc.response)
			}))
			defer ts.Close()

			c := newTestAIAuthoringService(ts.URL)
			got, err := c.IsAIAuthoringEnabled(context.Background(), tc.orgID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %t", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %t, want %t", got, tc.want)
			}
		})
	}
}
