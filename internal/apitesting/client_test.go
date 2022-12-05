package apitesting

import (
	"context"
	"github.com/saucelabs/saucectl/internal/config"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestClient_GetEventResult(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		eventID string
	}
	tests := []struct {
		name    string
		args    args
		want    TestResult
		wantErr bool
	}{
		{
			name: "passing test",
			args: args{
				hookID:  "dummyHookId",
				eventID: "completedEvent",
				ctx:     context.Background(),
			},
			want: TestResult{
				EventID:              "638e1e14a1da1e511c776eea",
				ExecutionTimeSeconds: 31,
				Async:                false,
				FailuresCount:        0,
				Project: Project{
					ID:   "6244d915ca28694aab958bbe",
					Name: "Test Project",
				},
				Test: Test{
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

	c := &Client{
		HTTPClient: ts.Client(),
		URL:        ts.URL,
		Username:   "dummy",
		AccessKey:  "accesskey",
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.GetEventResult(tt.args.ctx, tt.args.hookID, tt.args.eventID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEventResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEventResult() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetProject(t *testing.T) {
	type args struct {
		ctx    context.Context
		hookID string
	}
	tests := []struct {
		name    string
		args    args
		want    Project
		wantErr bool
	}{
		{
			args: args{ctx: context.Background(), hookID: "dummyProject"},
			want: Project{
				ID:   "6244d915ca28694aab000000",
				Name: "Test Project",
			},
			wantErr: false,
		},
		{
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
	c := &Client{
		HTTPClient: ts.Client(),
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
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_composeURL(t *testing.T) {
	type args struct {
		path    string
		buildID string
		format  string
		tunnel  config.Tunnel
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
			want: "/dummy/path?tunnelId=tunnelId%3AtunnelOwner",
		},
		{
			name: "Path with tunnel without owner",
			args: args{
				path: "/dummy/path",
				tunnel: config.Tunnel{
					Name: "tunnelId",
				},
			},
			want: "/dummy/path?tunnelId=tunnelId",
		},
	}
	c := &Client{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.composeURL(tt.args.path, tt.args.buildID, tt.args.format, tt.args.tunnel); got != tt.want {
				t.Errorf("composeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
