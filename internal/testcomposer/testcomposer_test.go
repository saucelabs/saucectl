package testcomposer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
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

			got, _, err := c.StartJob(tt.args.ctx, tt.args.jobStarterPayload)
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

func TestClient_GetSlackToken(t *testing.T) {
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
	tests := []struct {
		name       string
		fields     fields
		want       string
		wantErr    bool
		serverFunc func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:    "token exists",
			fields:  fields{HTTPClient: server.Client(), URL: server.URL},
			want:    "user token",
			wantErr: false,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				json.NewEncoder(w).Encode(TokenResponse{
					Token: "user token",
				})
			},
		},
		{
			name:    "token validation error",
			fields:  fields{HTTPClient: server.Client(), URL: server.URL},
			want:    "",
			wantErr: true,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(422)
			},
		},
		{
			name:    "token does not exists",
			fields:  fields{HTTPClient: server.Client(), URL: server.URL},
			want:    "",
			wantErr: true,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
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

			got, err := c.GetSlackToken(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSlackToken error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSlackToken got = %v, want %v", got, tt.want)
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
			want: framework.Metadata{
				FrameworkName:    "testycles",
				FrameworkVersion: "1",
				DockerImage:      "sauce/testycles:v1+v0.1.0",
				GitRelease:       "",
			},
			wantErr: false,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				json.NewEncoder(w).Encode(FrameworkResponse{
					Name:    "testycles",
					Version: "1",
					Runner: runner{
						DockerImage: "sauce/testycles:v1+v0.1.0",
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
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetImage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_UploadAsset(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/testcomposer/jobs/1/assets":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{\"uploaded\":null}"))
		case "/v1/testcomposer/jobs/2/assets":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{\"uploaded\":null,\"errors\":[\"failed to upload config.yml: content-type not allowed\"]}"))
		case "/v1/testcomposer/jobs/3/assets":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	// timeout := 3 * time.Second

	type args struct {
		jobID       string
		fileName    string
		contentType string
		content     []byte
	}
	tests := []struct {
		client  Client
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Valid case",
			client: Client{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: credentials.Credentials{Username: "test", AccessKey: "123"},
			},
			args: args{
				jobID:       "1",
				fileName:    "config.yml",
				contentType: "text/plain",
				content:     []byte("dummy-content"),
			},
			wantErr: false,
		},
		{
			name: "invalid case - 400",
			client: Client{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: credentials.Credentials{Username: "test", AccessKey: "123"},
			},
			args: args{
				jobID:       "2",
				fileName:    "config.yml",
				contentType: "text/plain",
				content:     []byte("dummy-content"),
			},
			wantErr: true,
		},
		{
			name: "invalid 404",
			client: Client{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: credentials.Credentials{Username: "test", AccessKey: "123"},
			},
			args: args{
				jobID:       "3",
				fileName:    "config.yml",
				contentType: "text/plain",
				content:     []byte("dummy-content"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.client.UploadAsset(tt.args.jobID, tt.args.fileName, tt.args.contentType, tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("UploadAsset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Frameworks(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		httpCode int
		want     []framework.Framework
		wantErr  bool
	}{
		{
			name:     "HTTP - 200",
			body:     `[{"name": "cypress"},{"name":"playwright"},{"name":"puppeteer"},{"name":"testcafe"},{"name":"espresso"},{"name":"xcuitest"}]`,
			httpCode: 200,
			want: []framework.Framework{
				{Name: "cypress"},
				{Name: "playwright"},
				{Name: "puppeteer"},
				{Name: "testcafe"},
				{Name: "espresso"},
				{Name: "xcuitest"},
			},
			wantErr: false,
		},
		{
			name:     "HTTP - 500",
			body:     ``,
			httpCode: 500,
			want:     []framework.Framework{},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/testcomposer/frameworks":
					w.WriteHeader(tt.httpCode)
					w.Write([]byte(tt.body))
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			c := &Client{
				HTTPClient:  http.DefaultClient,
				URL:         ts.URL,
				Credentials: credentials.Credentials{Username: "test", AccessKey: "123"},
			}
			got, err := c.Frameworks(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Frameworks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Frameworks() got = %v, want %v", got, tt.want)
			}
			ts.Close()
		})
	}
}

func TestClient_Versions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/testcomposer/frameworks/cypress/versions":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"name":"cypress","version":"7.3.0","deprecated":false,"runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v7.1.1","gitRelease":"saucelabs/sauce-cypress-runner:v7.1.1"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]},{"name":"cypress","version":"7.1.0","deprecated":true,"runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v7.0.6","gitRelease":"saucelabs/sauce-cypress-runner:v7.0.6"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]},{"name":"cypress","version":"6.6.0","deprecated":true,"runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v6.0.2","gitRelease":"saucelabs/sauce-cypress-runner:v6.0.2"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]}]`))
		case "/v1/testcomposer/frameworks/non-existent/versions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	c := &Client{
		HTTPClient:  ts.Client(),
		URL:         ts.URL,
		Credentials: credentials.Credentials{Username: "test", AccessKey: "123"},
	}
	type args struct {
		client        *Client
		frameworkName string
	}
	tests := []struct {
		name    string
		want    []framework.Metadata
		args    args
		wantErr bool
	}{
		{
			name: "HTTP - 200",
			args: args{
				frameworkName: "cypress",
			},
			want: []framework.Metadata{
				{
					FrameworkName:    "cypress",
					FrameworkVersion: "7.3.0",
					Deprecated:       false,
					GitRelease:       "saucelabs/sauce-cypress-runner:v7.1.1",
					DockerImage:      "saucelabs/stt-cypress-mocha-node:v7.1.1",
					Platforms: []framework.Platform{
						{
							PlatformName: "windows 10",
							BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
						},
					},
				}, {
					FrameworkName:    "cypress",
					FrameworkVersion: "7.1.0",
					Deprecated:       true,
					GitRelease:       "saucelabs/sauce-cypress-runner:v7.0.6",
					DockerImage:      "saucelabs/stt-cypress-mocha-node:v7.0.6",
					Platforms: []framework.Platform{
						{
							PlatformName: "windows 10",
							BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
						},
					},
				}, {
					FrameworkName:    "cypress",
					FrameworkVersion: "6.6.0",
					Deprecated:       true,
					GitRelease:       "saucelabs/sauce-cypress-runner:v6.0.2",
					DockerImage:      "saucelabs/stt-cypress-mocha-node:v6.0.2",
					Platforms: []framework.Platform{
						{
							PlatformName: "windows 10",
							BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
						},
					},
				},
			},
			wantErr: false,
		}, {
			name: "HTTP - 500",
			args: args{
				frameworkName: "buggy",
			},
			want:    []framework.Metadata{},
			wantErr: true,
		}, {
			name: "HTTP - 404",
			args: args{
				frameworkName: "Non-existent",
			},
			want:    []framework.Metadata{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Versions(context.Background(), tt.args.frameworkName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Versions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Versions() got = %v, want %v", got, tt.want)
			}
			ts.Close()
		})
	}
}
