package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/stretchr/testify/assert"
)

func TestBuildService_ListBuilds(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		responseBody []byte
		want         []build.Build
		wantErr      error
	}{
		{
			name:         "successful response",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"builds": [{"name":"My build name", "id":"1234", "status":"success"}]}`),
			want: []build.Build{
				{
					ID:     "1234",
					URL:    "https://app.saucelabs.com/builds/vdc/1234",
					Name:   "My build name",
					Status: build.StateSuccess,
				},
			},
			wantErr: nil,
		},
		{
			name:         "build not found",
			statusCode:   http.StatusNotFound,
			responseBody: nil,
			want:         []build.Build{},
			wantErr:      errors.New("unexpected statusCode: 404"),
		},
		{
			name:         "validation error",
			statusCode:   http.StatusUnprocessableEntity,
			responseBody: nil,
			want:         []build.Build{},
			wantErr:      errors.New("unexpected statusCode: 422"),
		},
		{
			name:         "unparseable response",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id": "bad-json-response"`),
			want:         []build.Build{},
			wantErr:      errors.New("unexpected EOF"),
		},
	}
	for _, tt := range testCases {
		t.Run(
			tt.name, func(t *testing.T) {
				// arrange
				ts := httptest.NewServer(
					http.HandlerFunc(
						func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(tt.statusCode)
							_, _ = w.Write(tt.responseBody)
						},
					),
				)
				defer ts.Close()

				client := NewBuildService(
					region.None, "user", "key", 3*time.Second,
				)
				client.URL = ts.URL
				client.AppURL = "https://app.saucelabs.com"
				client.Client.RetryWaitMax = 1 * time.Millisecond

				// act
				bid, err := client.ListBuilds(
					context.Background(), build.ListBuildsOptions{
						Source: build.SourceVDC,
					},
				)

				// assert
				assert.Equal(t, tt.want, bid)
				if err != nil {
					assert.True(
						t, strings.Contains(err.Error(), tt.wantErr.Error()),
					)
				}
			},
		)
	}
}

func TestBuildService_GetBuildID(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		responseBody []byte
		want         build.Build
		wantErr      error
	}{
		{
			name:         "happy case",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id": "happy-build-id"}`),
			want: build.Build{
				ID:  "happy-build-id",
				URL: "https://app.saucelabs.com/builds/vdc/happy-build-id",
			},
			wantErr: nil,
		},
		{
			name:         "job not found",
			statusCode:   http.StatusNotFound,
			responseBody: nil,
			want:         build.Build{},
			wantErr:      errors.New("unexpected statusCode: 404"),
		},
		{
			name:         "validation error",
			statusCode:   http.StatusUnprocessableEntity,
			responseBody: nil,
			want:         build.Build{},
			wantErr:      errors.New("unexpected statusCode: 422"),
		},
		{
			name:         "unparseable response",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id": "bad-json-response"`),
			want:         build.Build{},
			wantErr:      errors.New("unexpected EOF"),
		},
	}
	for _, tt := range testCases {
		t.Run(
			tt.name, func(t *testing.T) {
				// arrange
				ts := httptest.NewServer(
					http.HandlerFunc(
						func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(tt.statusCode)
							_, _ = w.Write(tt.responseBody)
						},
					),
				)
				defer ts.Close()

				client := NewBuildService(
					region.None, "user", "key", 3*time.Second,
				)
				client.URL = ts.URL
				client.AppURL = "https://app.saucelabs.com"
				client.Client.RetryWaitMax = 1 * time.Millisecond

				// act
				bid, err := client.GetBuild(
					context.Background(), build.GetBuildOptions{
						ID: "some-job-id", Source: build.SourceVDC, ByJob: true,
					},
				)

				// assert
				assert.Equal(t, tt.want, bid)
				if err != nil {
					assert.True(
						t, strings.Contains(err.Error(), tt.wantErr.Error()),
					)
				}
			},
		)
	}
}

func TestBuildService_GetBuildURL(t *testing.T) {
	testCases := []struct {
		name     string
		byJob    bool
		urlMatch string
	}{
		{
			name:     "Build by ID",
			urlMatch: "/v2/builds/(vdc|rdc)/\\w+/",
			byJob:    false,
		},
		{
			name:     "Build by Job ID",
			urlMatch: "/v2/builds/(vdc|rdc)/jobs/\\w+/build/",
			byJob:    true,
		},
	}

	for _, tt := range testCases {
		t.Run(
			tt.name, func(t *testing.T) {
				// arrange
				var reqURL string
				ts := httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						reqURL = req.URL.Path
						w.WriteHeader(200)
						_, _ = w.Write([]byte(`{}`))
					},
				))
				defer ts.Close()

				client := NewBuildService(
					region.None, "user", "key", 3*time.Second,
				)
				client.URL = ts.URL
				client.AppURL = "https://app.saucelabs.com"
				client.Client.RetryWaitMax = 1 * time.Millisecond

				// act
				_, _ = client.GetBuild(
					context.Background(), build.GetBuildOptions{
						ID: "1234", Source: build.SourceVDC, ByJob: tt.byJob,
					},
				)

				// assert
				assert.Regexp(t, tt.urlMatch, reqURL)
			},
		)
	}
}
