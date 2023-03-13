package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/job"
)

func TestTestComposer_StartJob(t *testing.T) {
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
				w.WriteHeader(201)
				_ = json.NewEncoder(w).Encode(struct {
					JobID string `json:"jobID"`
				}{
					JobID: "fake-job-id",
				})
			},
		},
		{
			name: "Non 2xx status code",
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
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    "",
			wantErr: fmt.Errorf("job start failed; unexpected response code:'500', msg:'Internal server error'"),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				_, err := w.Write([]byte("Internal server error"))
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

			c := &TestComposer{
				HTTPClient: server.Client(),
				URL:        server.URL,
			}

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

func TestTestComposer_GetSlackToken(t *testing.T) {
	type fields struct {
		HTTPClient  *http.Client
		URL         string
		Credentials iam.Credentials
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
			want:    "user token",
			wantErr: false,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				err := json.NewEncoder(w).Encode(TokenResponse{
					Token: "user token",
				})
				if err != nil {
					t.Errorf("failed to encode json response: %v", err)
				}
			},
		},
		{
			name:    "token validation error",
			want:    "",
			wantErr: true,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(422)
			},
		},
		{
			name:    "token does not exists",
			want:    "",
			wantErr: true,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			c := &TestComposer{
				HTTPClient:  server.Client(),
				URL:         server.URL,
				Credentials: tt.fields.Credentials,
			}

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

func TestTestComposer_Search(t *testing.T) {
	type fields struct {
		HTTPClient  *http.Client
		URL         string
		Credentials iam.Credentials
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
			name: "framework version available",
			args: args{context.Background(), framework.SearchOptions{
				Name:             "testycles",
				FrameworkVersion: "1",
			}},
			want: framework.Metadata{
				FrameworkName:    "testycles",
				FrameworkVersion: "1",
				EOLDate:          time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC),
				RemovalDate:      time.Date(2023, 04, 01, 0, 0, 0, 0, time.UTC),
				DockerImage:      "sauce/testycles:v1+v0.1.0",
				GitRelease:       "",
			},
			wantErr: false,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				err := json.NewEncoder(w).Encode(FrameworkResponse{
					Name:        "testycles",
					Version:     "1",
					EOLDate:     time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC),
					RemovalDate: time.Date(2023, 04, 01, 0, 0, 0, 0, time.UTC),
					Runner: runner{
						DockerImage: "sauce/testycles:v1+v0.1.0",
					},
				})
				if err != nil {
					t.Errorf("failed to encode json response: %v", err)
				}
			},
		},
		{
			name: "unknown framework or version",
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
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			c := &TestComposer{
				HTTPClient:  server.Client(),
				URL:         server.URL,
				Credentials: tt.fields.Credentials,
			}

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

func TestTestComposer_UploadAsset(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/testcomposer/jobs/1/assets":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("{\"uploaded\":null}"))
		case "/v1/testcomposer/jobs/2/assets":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("{\"uploaded\":null,\"errors\":[\"failed to upload config.yml: content-type not allowed\"]}"))
		case "/v1/testcomposer/jobs/3/assets":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
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
		client  TestComposer
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Valid case",
			client: TestComposer{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: iam.Credentials{Username: "test", AccessKey: "123"},
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
			client: TestComposer{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: iam.Credentials{Username: "test", AccessKey: "123"},
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
			client: TestComposer{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: iam.Credentials{Username: "test", AccessKey: "123"},
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
			if err := tt.client.UploadAsset(tt.args.jobID, false, tt.args.fileName, tt.args.contentType, tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("UploadAsset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTestComposer_Frameworks(t *testing.T) {
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
				var err error
				switch r.URL.Path {
				case "/v1/testcomposer/frameworks":
					w.WriteHeader(tt.httpCode)
					_, err = w.Write([]byte(tt.body))
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
				if err != nil {
					t.Errorf("failed to respond: %v", err)
				}
			}))
			c := &TestComposer{
				HTTPClient:  http.DefaultClient,
				URL:         ts.URL,
				Credentials: iam.Credentials{Username: "test", AccessKey: "123"},
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

func TestTestComposer_Versions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/testcomposer/frameworks/cypress/versions":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`[{"name":"cypress","version":"7.3.0","eolDate":"2023-01-01T00:00:00Z","removalDate":"2023-04-01T00:00:00Z","runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v7.1.1","gitRelease":"saucelabs/sauce-cypress-runner:v7.1.1"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]},{"name":"cypress","version":"7.1.0","eolDate":"2023-01-01T00:00:00Z","removalDate":"2023-04-01T00:00:00Z","runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v7.0.6","gitRelease":"saucelabs/sauce-cypress-runner:v7.0.6"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]},{"name":"cypress","version":"6.6.0","eolDate":"2023-01-01T00:00:00Z","removalDate":"2023-04-01T00:00:00Z","runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v6.0.2","gitRelease":"saucelabs/sauce-cypress-runner:v6.0.2"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]}]`))
		case "/v1/testcomposer/frameworks/non-existent/versions":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &TestComposer{
		HTTPClient:  ts.Client(),
		URL:         ts.URL,
		Credentials: iam.Credentials{Username: "test", AccessKey: "123"},
	}
	type args struct {
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
					EOLDate:          time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC),
					RemovalDate:      time.Date(2023, 04, 01, 0, 0, 0, 0, time.UTC),
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
					EOLDate:          time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC),
					RemovalDate:      time.Date(2023, 04, 01, 0, 0, 0, 0, time.UTC),
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
					EOLDate:          time.Date(2023, 01, 01, 0, 0, 0, 0, time.UTC),
					RemovalDate:      time.Date(2023, 04, 01, 0, 0, 0, 0, time.UTC),
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
