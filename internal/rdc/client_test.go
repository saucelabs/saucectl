package rdc

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
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
		assert.Equal(t, tt.wantErr, err)
		assert.Equal(t, jb, tt.want)
	}
}


func randJobStatus(j *job.Job, isComplete bool) {
	min := 1
	max := 10
	randNum := rand.Intn(max-min+1) + min

	status := "error"
	if isComplete {
		status = "complete"
	}

	if randNum >= 5 {
		j.Status = status
	}
}

func TestClient_GetJobStatus(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/rdc/jobs/1":
			details := &job.Job{
				ID:     "1",
				Passed: false,
				Status: "new",
				Error:  "",
			}
			randJobStatus(details, true)

			resp, _ := json.Marshal(details)
			w.Write(resp)
		case "/v1/rdc/jobs/2":
			details := &job.Job{
				ID:     "2",
				Passed: false,
				Status: "in progress",
				Error:  "User Abandoned Test -- User terminated",
			}
			randJobStatus(details, false)

			resp, _ := json.Marshal(details)
			w.Write(resp)
		case "/v1/rdc/jobs/3":
			w.WriteHeader(http.StatusNotFound)
		case "/v1/rdc/jobs/4":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	timeout := 3 * time.Second

	testCases := []struct {
		name         string
		client       Client
		jobID        string
		expectedResp job.Job
		expectedErr  error
	}{
		{
			name:   "get job details with ID 1 and status 'complete'",
			client: New(ts.URL, "test", "123", timeout),
			jobID:  "1",
			expectedResp: job.Job{
				ID:     "1",
				Passed: false,
				Status: "complete",
				Error:  "",
			},
			expectedErr: nil,
		},
		{
			name:   "get job details with ID 2 and status 'error'",
			client: New(ts.URL, "test", "123", timeout),
			jobID:  "2",
			expectedResp: job.Job{
				ID:     "2",
				Passed: false,
				Status: "error",
				Error:  "User Abandoned Test -- User terminated",
			},
			expectedErr: nil,
		},
		{
			name:         "user not found error from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "3",
			expectedResp: job.Job{},
			expectedErr:  ErrJobNotFound,
		},
		{
			name:         "http status is not 200, but 401 from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "4",
			expectedResp: job.Job{},
			expectedErr:  errors.New("job status request failed; unexpected response code:'401', msg:''"),
		},
		{
			name:         "unexpected status code from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "333",
			expectedResp: job.Job{},
			expectedErr:  ErrServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.client.PollJob(context.Background(), tc.jobID, 10*time.Millisecond)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedResp, got)
		})
	}
}