package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func createTestRetryableHTTPClient(t *testing.T) *retryablehttp.Client {
	return &retryablehttp.Client{
		HTTPClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		RetryWaitMin: 0 * time.Second,
		RetryWaitMax: 0 * time.Second,
		RetryMax:     1,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
		ErrorHandler: retryablehttp.PassthroughErrorHandler,
	}
}

func TestAPITester_GetEventResult(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		eventID string
	}
	tests := []struct {
		name    string
		args    args
		want    apitest.TestResult
		wantErr bool
	}{
		{
			name: "passing test",
			args: args{
				hookID:  "dummyHookId",
				eventID: "completedEvent",
				ctx:     context.Background(),
			},
			want: apitest.TestResult{
				EventID:              "638e1e14a1da1e511c776eea",
				ExecutionTimeSeconds: 31,
				Async:                false,
				FailuresCount:        0,
				Project: apitest.ProjectMeta{
					ID:   "6244d915ca28694aab958bbe",
					Name: "Test Project",
				},
				Test: apitest.Test{
					ID:   "638788b12d29c47170999eee",
					Name: "test_demo",
				},
			},
			wantErr: false,
		},
		{
			name: "404 Event",
			args: args{
				hookID:  "dummyHookId",
				eventID: "incompleteEvent",
				ctx:     context.Background(),
			},
			wantErr: true,
		},
		{
			name: "Buggy Event",
			args: args{
				hookID:  "dummyHookId",
				eventID: "buggyEvent",
				ctx:     context.Background(),
			},
			wantErr: true,
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/insights/events/completedEvent":
			completeStatusResp := []byte(`{"_id":"638e1e14a1da1e511c776eea","events":[],"tags":["canfail"],"criticalFailures":[],"httpFailures":[],"facts":{},"date":1670258196613,"test":{"name":"test_demo","id":"638788b12d29c47170999eee"},"failuresCount":0,"warningsCount":0,"compressed":false,"run":{"name":"","id":""},"company":{"name":"","id":"7fb25570b4064716b9b6daae1a997bba"},"project":{"name":"Test Project","id":"6244d915ca28694aab958bbe"},"temp":false,"expireAt":"2023-06-06T04:37:07Z","executionTimeSeconds":31,"taskId":"ad24fdd6-8e47-401c-81ce-866553194bdd","agent":"wstestjs","mode":"ondemand","buildId":"Test","clientname":"","initiator":{"name":"Incitator","id":"de8691a22ff343f08aa6fb63e485fe0d","teamid":"0205cb60678a4372193bac4052c048be"}}`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/rest/v4/dummyHookId/insights/events/incompleteEvent":
			errorStatusResp := []byte(`{"status": "error","message": "event not found"}`)
			w.WriteHeader(http.StatusNotFound)
			_, err = w.Write(errorStatusResp)
		case "/api-testing/rest/v4/dummyHookId/insights/events/unauthorized":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()

	c := &APITester{
		HTTPClient:         createTestRetryableHTTPClient(t),
		URL:                ts.URL,
		Username:           "dummy",
		AccessKey:          "accesskey",
		RequestRateLimiter: rate.NewLimiter(rate.Inf, 0),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.GetEventResult(tt.args.ctx, tt.args.hookID, tt.args.eventID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEventResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("GetEventResult() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPITester_GetProject(t *testing.T) {
	type args struct {
		ctx    context.Context
		hookID string
	}
	tests := []struct {
		name    string
		args    args
		want    apitest.ProjectMeta
		wantErr bool
	}{
		{
			name: "Passing Project Fetch",
			args: args{ctx: context.Background(), hookID: "dummyProject"},
			want: apitest.ProjectMeta{
				ID:   "6244d915ca28694aab000000",
				Name: "Test Project",
			},
			wantErr: false,
		},
		{
			name:    "Failing Project Fetch",
			args:    args{ctx: context.Background(), hookID: "nonExistingProject"},
			wantErr: true,
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyProject":
			completeStatusResp := []byte(`{"id":"6244d915ca28694aab000000","name":"Test Project","teamId":"0205cb60678a4372b9ee20408725467ad","description":"","tags":[],"notes":"","type":"project","emailNotifications":[],"connectorNotifications":[]}`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/rest/v4/nonExistingProject":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &APITester{
		HTTPClient: createTestRetryableHTTPClient(t),
		URL:        ts.URL,
		Username:   "dummy",
		AccessKey:  "accesskey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.GetProject(tt.args.ctx, tt.args.hookID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("GetProject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPITester_GetTest(t *testing.T) {
	type args struct {
		ctx    context.Context
		hookID string
		testID string
	}
	tests := []struct {
		name    string
		args    args
		want    apitest.Test
		wantErr bool
	}{
		{
			name: "Passing Test Fetch",
			args: args{ctx: context.Background(), hookID: "dummyProject", testID: "existingTest"},
			want: apitest.Test{
				ID:   "638788b12d29c47170d20db4",
				Name: "test_cli",
			},
			wantErr: false,
		},
		{
			name:    "Failing test fetch",
			args:    args{ctx: context.Background(), hookID: "dummyProject", testID: "nonexistentTest"},
			wantErr: true,
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyProject/tests/existingTest":
			completeStatusResp := []byte(`{"published":{"id":"638788b12d29c47170d20db4","name":"test_cli","description":"","lastModified":"2022-11-30T17:20:08Z","tags":["canfail"],"user":{"id":"de8691a22ff343f08aa6fb63e963121d","name":"Username"},"unit":"assertions:\n  - id: get\n    children:\n      - id: header\n        name: x-rapidmock-delay\n        value: \"10000\"\n    url: https://api.rapidmock.com/mocks/f6GeB\n    var: payload\n    mode: json\n  - id: if\n    children:\n      - id: comment\n        text: endpoint is not working fine, test will be stopped\n      - id: flow\n        command: stop\n    expression: payload_response.statusCode!='200'\nconfigs: []","input":"- id: global\n  children:\n    - id: variable\n      name: protocol\n      value: http://\n    - id: variable\n      name: domain\n      value: demoapi.apifortress.com\n    - id: variable\n      name: endpoint\n      value: /api/retail/product/${id}\n    - id: variable\n      name: auth\n      value: ABC123\n- id: sets\n  children:\n    - id: set\n      children:\n        - id: variable\n          name: id\n          value: \"1\"\n      name: product 1\n    - id: set\n      children:\n        - id: variable\n          name: id\n          value: \"4\"\n      name: product 2\n    - id: set\n      children:\n        - id: variable\n          name: id\n          value: \"7\"\n      name: product 3","complete":true},"workingCopy":{"id":"638790c8e90a3c46b5c83a98","user":{"id":"de8691a22ff343f08aa6fb63e963121d","name":"Username"},"unit":"assertions:\n  - id: get\n    children:\n      - id: header\n        name: x-rapidmock-delay\n        value: \"10000\"\n    url: https://api.rapidmock.com/mocks/f6GeB\n    var: payload\n    mode: json\n  - id: if\n    children:\n      - id: comment\n        text: endpoint is not working fine, test will be stopped\n      - id: flow\n        command: stop\n    expression: payload_response.statusCode!='200'\nconfigs: []","input":"- id: global\n  children:\n    - id: variable\n      name: protocol\n      value: http://\n    - id: variable\n      name: domain\n      value: demoapi.apifortress.com\n    - id: variable\n      name: endpoint\n      value: /api/retail/product/${id}\n    - id: variable\n      name: auth\n      value: ABC123\n- id: sets\n  children:\n    - id: set\n      children:\n        - id: variable\n          name: id\n          value: \"1\"\n      name: product 1\n    - id: set\n      children:\n        - id: variable\n          name: id\n          value: \"4\"\n      name: product 2\n    - id: set\n      children:\n        - id: variable\n          name: id\n          value: \"7\"\n      name: product 3","lastModified":"2022-11-30T17:20:08Z"}}`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/rest/v4/dummyProject/tests/nonexistentTest":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &APITester{
		HTTPClient: createTestRetryableHTTPClient(t),
		URL:        ts.URL,
		Username:   "dummy",
		AccessKey:  "accesskey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.GetTest(tt.args.ctx, tt.args.hookID, tt.args.testID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("GetProject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPITester_composeURL(t *testing.T) {
	type args struct {
		path    string
		buildID string
		format  string
		tunnel  config.Tunnel
		taskID  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Default Path",
			args: args{
				path: "/dummy/path",
			},
			want: "/dummy/path",
		},
		{
			name: "Path with buildId",
			args: args{
				path:    "/dummy/path",
				buildID: "buildId",
			},
			want: "/dummy/path?buildId=buildId",
		},
		{
			name: "Path with buildId and Format",
			args: args{
				path:    "/dummy/path",
				buildID: "buildId",
				format:  "json",
			},
			want: "/dummy/path?buildId=buildId&format=json",
		},
		{
			name: "Path with Format",
			args: args{
				path:   "/dummy/path",
				format: "json",
			},
			want: "/dummy/path?format=json",
		},
		{
			name: "Path with tunnel with owner",
			args: args{
				path: "/dummy/path",
				tunnel: config.Tunnel{
					Name:  "tunnelId",
					Owner: "tunnelOwner",
				},
			},
			want: "/dummy/path?tunnelId=tunnelOwner%3AtunnelId",
		},
		{
			name: "Path with tunnel without owner",
			args: args{
				path: "/dummy/path",
				tunnel: config.Tunnel{
					Name: "tunnelId",
				},
			},
			want: "/dummy/path?tunnelId=dummyUsername%3AtunnelId",
		},
		{
			name: "Path with taskId",
			args: args{
				path:   "/dummy/path",
				taskID: "taskId",
			},
			want: "/dummy/path?taskId=taskId",
		},
	}
	c := &APITester{
		Username: "dummyUsername",
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.composeURL(tt.args.path, tt.args.buildID, tt.args.format, tt.args.tunnel, tt.args.taskID); got != tt.want {
				t.Errorf("composeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPITester_GetProjects(t *testing.T) {
	tests := []struct {
		name    string
		want    []apitest.ProjectMeta
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Fetching Projects Test",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return err != nil
			},
			want: []apitest.ProjectMeta{},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/api-testing/api/project":
			completeStatusResp := []byte(`[{"id":"63dbe9d6f48c8412fe79220d","name":"Demo Project","teamId":null,"description":"","tags":[],"notes":"","type":"project","emailNotifications":[],"connectorNotifications":[]}]`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()

	c := &APITester{
		HTTPClient: createTestRetryableHTTPClient(t),
		URL:        ts.URL,
		Username:   "dummy",
		AccessKey:  "accesskey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.GetProjects(context.Background())
			if !tt.wantErr(t, err, fmt.Sprintf("GetProjects(%v)", context.Background())) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetProjects(%v)", context.Background())
		})
	}
}

func TestAPITester_GetHooks(t *testing.T) {
	type params struct {
		projectID string
	}

	tests := []struct {
		name    string
		params  params
		want    []apitest.Hook
		wantErr error
	}{
		{
			name: "Projects with no hooks",
			params: params{
				projectID: "noHooks",
			},
			wantErr: nil,
			want:    []apitest.Hook{},
		},
		{
			name: "Projects with multiple hooks",
			params: params{
				projectID: "multipleHooks",
			},
			wantErr: nil,
			want: []apitest.Hook{
				{
					Identifier: "e291c7c5-d091-4bae-8293-7315fc15cc4c",
					Name:       "name1",
				},
				{
					Identifier: "4d66f4d0-a29a-43a1-a787-94f7b8cc2e21",
					Name:       "name2",
				},
			},
		},
		{
			name: "Invalid Project",
			params: params{
				projectID: "invalidProject",
			},
			wantErr: errors.New(`request failed; unexpected response code:'404', msg:'{"status":"error","message":"Not Found"}'`),
			want:    []apitest.Hook{},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/api-testing/api/project/noHooks/hook":
			completeStatusResp := []byte(`[]`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/api/project/multipleHooks/hook":
			completeStatusResp := []byte(`[{"id":"hook1","identifier":"e291c7c5-d091-4bae-8293-7315fc15cc4c","name":"name1","description":"description1"},{"id":"hook2","identifier":"4d66f4d0-a29a-43a1-a787-94f7b8cc2e21","name":"name2","description":"description2"}]`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/api/project/invalidProject/hook":
			completeStatusResp := []byte(`{"status":"error","message":"Not Found"}`)
			w.WriteHeader(http.StatusNotFound)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()

	c := &APITester{
		HTTPClient: createTestRetryableHTTPClient(t),
		URL:        ts.URL,
		Username:   "dummy",
		AccessKey:  "accesskey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.GetHooks(context.Background(), tt.params.projectID)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("GetHooks(%v, %s): got %v want %v", context.Background(), tt.params.projectID, err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "GetHooks(%v, %s)", context.Background(), tt.params.projectID)
		})
	}
}

func TestAPITester_RunAllAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		buildID string
		tunnel  config.Tunnel
	}
	tests := []struct {
		name    string
		args    args
		want    apitest.AsyncResponse
		wantErr bool
	}{
		{
			name: "Basic trigger",
			args: args{
				ctx:    context.Background(),
				hookID: "dummyHookId",
			},
			want: apitest.AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/tests/_run-all":
			completeStatusResp := []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &APITester{
		HTTPClient: createTestRetryableHTTPClient(t),
		URL:        ts.URL,
		Username:   "dummyUser",
		AccessKey:  "dummyAccesKey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.RunAllAsync(tt.args.ctx, tt.args.hookID, tt.args.buildID, tt.args.tunnel, apitest.TestRequest{})
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPITester_RunEphemeralAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		buildID string
		tunnel  config.Tunnel
		test    apitest.TestRequest
	}
	tests := []struct {
		name          string
		args          args
		assertRequest func(t *testing.T, r *http.Request)
		reply         []byte
		want          apitest.AsyncResponse
		wantErr       bool
	}{
		{
			name: "Complete Trigger",
			args: args{
				ctx:     context.Background(),
				hookID:  "dummyHookId",
				buildID: "generatedBuildId",
				tunnel:  config.Tunnel{Name: "tunnelId"},
				test:    apitest.TestRequest{},
			},
			assertRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/api-testing/rest/v4/dummyHookId/tests/_exec?buildId=generatedBuildId&tunnelId=dummyUser%3AtunnelId", r.RequestURI)
			},
			reply: []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`),
			want: apitest.AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusNotImplemented)
					return
				}
				switch r.URL.Path {
				case "/api-testing/rest/v4/dummyHookId/tests/_exec":
					tt.assertRequest(t, r)
					completeStatusResp := tt.reply
					_, _ = w.Write(completeStatusResp)
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer ts.Close()
			c := &APITester{
				HTTPClient: createTestRetryableHTTPClient(t),
				URL:        ts.URL,
				Username:   "dummyUser",
				AccessKey:  "dummyAccesKey",
			}

			got, err := c.RunEphemeralAsync(tt.args.ctx, tt.args.hookID, tt.args.buildID, tt.args.tunnel, tt.args.test)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPITester_RunTestAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		testID  string
		hookID  string
		buildID string
		tunnel  config.Tunnel
	}
	tests := []struct {
		name    string
		args    args
		want    apitest.AsyncResponse
		wantErr bool
	}{
		{
			name: "Basic trigger",
			args: args{
				ctx:    context.Background(),
				hookID: "dummyHookId",
				testID: "c20ad4d76fe97759aa27a0c9",
			},
			want: apitest.AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/tests/c20ad4d76fe97759aa27a0c9/_run":
			completeStatusResp := []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &APITester{
		HTTPClient: createTestRetryableHTTPClient(t),
		URL:        ts.URL,
		Username:   "dummyUser",
		AccessKey:  "dummyAccesKey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.RunTestAsync(tt.args.ctx, tt.args.hookID, tt.args.testID, tt.args.buildID, tt.args.tunnel, apitest.TestRequest{})
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPITester_RunTagAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		buildID string
		tagID   string
		tunnel  config.Tunnel
	}
	tests := []struct {
		name    string
		args    args
		want    apitest.AsyncResponse
		wantErr bool
	}{
		{
			name: "Basic trigger",
			args: args{
				ctx:    context.Background(),
				hookID: "dummyHookId",
				tagID:  "dummyTag",
			},
			want: apitest.AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/tests/_tag/dummyTag/_run":
			completeStatusResp := []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &APITester{
		HTTPClient: createTestRetryableHTTPClient(t),
		URL:        ts.URL,
		Username:   "dummyUser",
		AccessKey:  "dummyAccesKey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.RunTagAsync(tt.args.ctx, tt.args.hookID, tt.args.tagID, tt.args.buildID, tt.args.tunnel, apitest.TestRequest{})
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}
