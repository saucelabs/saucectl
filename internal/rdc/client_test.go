package rdc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/stretchr/testify/assert"
)

func TestClient_ReadAllowedCCY(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		responseBody []byte
		want         int
		wantErr      error
	}{
		{
			name:         "default case",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"organization": { "current": 0, "maximum": 2 }}`),
			want:         2,
			wantErr:      nil,
		},
		{
			name:         "invalid parsing",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"organization": { "current": 0, "maximum": 2`),
			want:         0,
			wantErr:      errors.New("unexpected EOF"),
		},
		{
			name:       "Forbidden endpoint",
			statusCode: http.StatusForbidden,
			want:       0,
			wantErr:    errors.New("unexpected statusCode: 403"),
		},
		{
			name:       "error endpoint",
			statusCode: http.StatusInternalServerError,
			want:       0,
			wantErr:    errors.New("unexpected statusCode: 500"),
		},
	}

	timeout := 3 * time.Second
	for _, tt := range testCases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.statusCode)
			w.Write(tt.responseBody)
		}))

		client := New(ts.URL, "test", "123", timeout)
		ccy, err := client.ReadAllowedCCY(context.Background())
		assert.Equal(t, err, tt.wantErr)
		assert.Equal(t, ccy, tt.want)
		ts.Close()
	}
}

func TestClient_ReadJob(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/rdc/jobs/test1":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"error": null, "status": "passed", "consolidated_status": "passed"}`))
		case "/v1/rdc/jobs/test2":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"error": "no-device-found", "status": "failed", "consolidated_status": "failed"}`))
		case "/v1/rdc/jobs/test3":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"error": null, "status": "in progress", "consolidated_status": "in progress"}`))
		case "/v1/rdc/jobs/test4":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	timeout := 3 * time.Second
	client := New(ts.URL, "test-user", "test-key", timeout)

	testCases := []struct {
		name    string
		jobID   string
		want    job.Job
		wantErr error
	}{
		{
			name: "passed job",
			jobID: "test1",
			want: job.Job{ID: "test1", Error: "", Status: "passed", Passed: true},
			wantErr: nil,
		},
		{
			name: "failed job",
			jobID: "test2",
			want: job.Job{ID: "test2", Error: "no-device-found", Status: "failed", Passed: false},
			wantErr: nil,
		},
		{
			name: "in progress job",
			jobID: "test3",
			want: job.Job{ID: "test3", Error: "", Status: "in progress", Passed: false},
			wantErr: nil,
		},
		{
			name: "non-existant job",
			jobID: "test4",
			want: job.Job{ID: "test4", Error: "", Status: "", Passed: false},
			wantErr: errors.New("unexpected statusCode: 404"),
		},
	}

	for _, tt := range testCases {
		job, err := client.ReadJob(context.Background(), tt.jobID)
		assert.Equal(t, err, tt.wantErr)
		if err == nil {
			assert.True(t, reflect.DeepEqual(job, tt.want))
		}
	}
}

func TestClient_StartJob(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"test_report": {"id": "test1","url": "https://app.staging.saucelabs.net/tests/test1"}}`))
	}))
	defer ts.Close()
	timeout := 3 * time.Second


	client := New(ts.URL, "test-user", "test-access-key", timeout)
	testCases := []struct {
		name    string
		options job.RDCStarterOptions
		want    string
		wantErr error
	}{
		{
			name: "Working Case",
			options: job.RDCStarterOptions{
				AppID:         "dummy-id.apk",
				TestAppID:     "dummy-test.apk",
				TestFramework: "ANDROID_INSTRUMENTATION",
				TestName:      "Working Case",
				DeviceQuery: job.RDCDeviceQuery{
					Type: job.RDCTypeDynamicDeviceQuery,
				},
				TestOptions: map[string]string{},
			},
			want: "test1",
		},
	}

	for _, tt := range testCases {
		jb, err := client.StartJob(tt.options)
		fmt.Printf("%v", jb)
		fmt.Printf("%v", tt.want)
		assert.Equal(t, tt.wantErr, err)
		assert.Equal(t, jb, tt.want)
	}
}
