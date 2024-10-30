package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/saucelabs/saucectl/internal/job"
)

func TestWebdriver_StartJob(t *testing.T) {
	type args struct {
		ctx               context.Context
		jobStarterPayload job.StartOptions
	}
	type fields struct {
		HTTPClient *http.Client
		URL        string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       job.Job
		wantErr    error
		serverFunc func(w http.ResponseWriter, r *http.Request) // what shall the mock server respond with
	}{
		{
			name: "Happy path",
			args: args{
				ctx: context.TODO(),
				jobStarterPayload: job.StartOptions{
					User:        "fake-user",
					AccessKey:   "fake-access-key",
					BrowserName: "fake-browser-name",
					Name:        "fake-test-name",
					Framework:   "fake-framework",
					Build:       "fake-buildname",
					Tags:        nil,
				},
			},
			want: job.Job{
				ID:     "fake-job-id",
				Status: job.StateInProgress,
				URL:    "/tests/fake-job-id",
			},
			wantErr: nil,
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(201)
				_ = json.NewEncoder(w).Encode(sessionStartResponse{
					SessionID: "fake-job-id",
				})
			},
		},
		{
			name: "Non 2xx status code",
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    job.Job{},
			wantErr: fmt.Errorf("job start failed (401): go away"),
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(401)
				_, _ = w.Write([]byte("go away"))
			},
		},
		{
			name: "Unknown error",
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    job.Job{},
			wantErr: fmt.Errorf("job start failed (500): internal server error"),
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
				_, err := w.Write([]byte("internal server error"))
				if err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			c := &Webdriver{
				HTTPClient: server.Client(),
				URL:        server.URL,
			}

			got, err := c.StartJob(tt.args.ctx, tt.args.jobStarterPayload)
			if (err != nil) && !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("StartJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("StartJob() (-want +got): \n%s", diff)
			}
		})
	}
}
