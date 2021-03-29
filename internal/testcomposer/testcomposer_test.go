package testcomposer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type Responder struct {
	Index   int
	Records []func(w http.ResponseWriter, r *http.Request)
	Test    *testing.T
}

func (r *Responder) Record(resFunc func(w http.ResponseWriter, req *http.Request)) {
	r.Records = append(r.Records, resFunc)
}

func (r *Responder) Play(w http.ResponseWriter, req *http.Request) {
	if r.Index >= len(r.Records) {
		r.Test.Errorf("responder requested more times than it has available records")
	}

	r.Records[r.Index](w, req)
	r.Index++
}

func respondJSON(w http.ResponseWriter, v interface{}, httpStatus int) {
	w.WriteHeader(httpStatus)
	b, err := json.Marshal(v)

	if err != nil {
		log.Err(err).Msg("failed to marshal job json")
		http.Error(w, "failed to marshal job json", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(b); err != nil {
		log.Err(err).Msg("Failed to write out response")
	}
}

func TestTestComposer_StartJob(t *testing.T) {
	respo := Responder{
		Test: t,
	}
	mockTestComposerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respo.Play(w, r)
	}))
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
		want       string
		wantErr    error
		serverFunc func(w http.ResponseWriter, r *http.Request) // what shall the mock server respond with
	}{
		{
			name: "Happy path",
			fields: fields{
				HTTPClient: mockTestComposerServer.Client(),
				URL:        mockTestComposerServer.URL,
			},
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
			want:    "fake-job-id",
			wantErr: nil,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				respondJSON(w, struct {
					JobID string `json:"jobID"`
				}{
					JobID: "fake-job-id",
				}, 201)
			},
		},
		{
			name: "Non 2xx status code",
			fields: fields{
				HTTPClient: mockTestComposerServer.Client(),
				URL:        mockTestComposerServer.URL,
			},
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    "",
			wantErr: fmt.Errorf("job start failed; unexpected response code:'300', msg:''"),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(300)
			},
		},
		{
			name: "Unknown error",
			fields: fields{
				HTTPClient: mockTestComposerServer.Client(),
				URL:        mockTestComposerServer.URL,
			},
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    "",
			wantErr: fmt.Errorf("job start failed; unexpected response code:'500', msg:'Internal server error'"),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				w.Write([]byte("Internal server error"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				HTTPClient: tt.fields.HTTPClient,
				URL:        tt.fields.URL,
			}

			respo.Record(tt.serverFunc)

			got, err := c.StartJob(tt.args.ctx, tt.args.jobStarterPayload)
			if (err != nil) && !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("StartJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StartJob() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_Search(t *testing.T) {
	respo := Responder{
		Test: t,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respo.Play(w, r)
	}))
	defer server.Close()

	type fields struct {
		HTTPClient  *http.Client
		URL         string
		Credentials credentials.Credentials
	}
	type args struct {
		ctx  context.Context
		opts framework.SearchOptions
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       framework.Metadata
		wantErr    bool
		serverFunc func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:   "framework version available",
			fields: fields{HTTPClient: server.Client(), URL: server.URL},
			args: args{context.Background(), framework.SearchOptions{
				Name:             "testycles",
				FrameworkVersion: "1",
			}},
			want:    framework.Metadata{
				FrameworkName:      "testycles",
				FrameworkVersion:   "1",
				CloudRunnerVersion: "0.1.0",
				DockerImage:        "sauce/testycles:v1+v0.1.0",
				GitRelease:         "",
			},
			wantErr: false,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				json.NewEncoder(w).Encode(FrameworkResponse{
					Name:    "testycles",
					Version: "1",
					Runner: runner{
						CloudRunnerVersion: "0.1.0",
						DockerImage:        "sauce/testycles:v1+v0.1.0",
					},
				})
			},
		},
		{
			name:   "unknown framework or version",
			fields: fields{HTTPClient: server.Client(), URL: server.URL},
			args: args{context.Background(), framework.SearchOptions{
				Name:             "notestycles",
				FrameworkVersion: "1",
			}},
			want:    framework.Metadata{},
			wantErr: true,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(400)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				HTTPClient:  tt.fields.HTTPClient,
				URL:         tt.fields.URL,
				Credentials: tt.fields.Credentials,
			}

			respo.Record(tt.serverFunc)

			got, err := c.Search(tt.args.ctx, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetImage() got = %v, want %v", got, tt.want)
			}
		})
	}
}
