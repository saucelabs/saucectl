package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/iam"
)

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

	client := NewRetryableClient(3 * time.Second)
	client.RetryMax = 0

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
				HTTPClient:  client,
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
				HTTPClient:  client,
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
				HTTPClient:  client,
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
				t.Errorf("UploadArtifact() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTestComposer_Frameworks(t *testing.T) {
	client := NewRetryableClient(3 * time.Second)
	client.RetryMax = 0

	tests := []struct {
		name     string
		body     string
		httpCode int
		want     []string
		wantErr  bool
	}{
		{
			name:     "HTTP - 200",
			body:     `[{"name":"cypress","version":"12.6.0"},{"name":"cypress","version":"12.3.0"},{"name":"playwright","version":"1.31.1"},{"name":"playwright","version":"1.29.2"},{"name":"puppeteer-replay","version":"0.8.0"},{"name":"puppeteer-replay","version":"0.7.0"},{"name":"testcafe","version":"2.1.0"},{"name":"testcafe","version":"2.0.1"}]`,
			httpCode: 200,
			want: []string{
				"cypress",
				"playwright",
				"puppeteer-replay",
				"testcafe",
			},
			wantErr: false,
		},
		{
			name:     "HTTP - 500",
			body:     ``,
			httpCode: 500,
			want:     []string{},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				switch r.URL.Path {
				case "/v2/testcomposer/frameworks":
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
				HTTPClient:  client,
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
	client := NewRetryableClient(3 * time.Second)
	client.RetryMax = 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.RequestURI {
		case "/v2/testcomposer/frameworks?frameworkName=cypress":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`[{"name":"cypress","version":"7.3.0","eolDate":"2023-01-01T00:00:00Z","removalDate":"2023-04-01T00:00:00Z","runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v7.1.1","gitRelease":"saucelabs/sauce-cypress-runner:v7.1.1"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]},{"name":"cypress","version":"7.1.0","eolDate":"2023-01-01T00:00:00Z","removalDate":"2023-04-01T00:00:00Z","runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v7.0.6","gitRelease":"saucelabs/sauce-cypress-runner:v7.0.6"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]},{"name":"cypress","version":"6.6.0","eolDate":"2023-01-01T00:00:00Z","removalDate":"2023-04-01T00:00:00Z","runner":{"cloudRunnerVersion":"","dockerImage":"saucelabs/stt-cypress-mocha-node:v6.0.2","gitRelease":"saucelabs/sauce-cypress-runner:v6.0.2"},"platforms":[{"name":"windows 10","browsers":["googlechrome","firefox","microsoftedge"]}]}]`))
		case "/v2/testcomposer/frameworks?frameworkName=non-existent":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()

	c := &TestComposer{
		HTTPClient:  client,
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
			name: "HTTP - Non-existent framework",
			args: args{
				frameworkName: "non-existent",
			},
			wantErr: false,
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
		})
	}
}
