package resto

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/job"
)

func TestClient_GetJobDetails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/v1/test/jobs/1":
			completeStatusResp := []byte(`{"browser_short_version": "85", "video_url": "https://localhost/jobs/1/video.mp4", "creation_time": 1605637528, "custom-data": null, "browser_version": "85.0.4183.83", "owner": "test", "automation_backend": "webdriver", "id": "1", "collects_automator_log": false, "record_screenshots": true, "record_video": true, "build": null, "passed": null, "public": "team", "assigned_tunnel_id": null, "status": "complete", "log_url": "https://localhost/jobs/1/selenium-server.log", "start_time": 1605637528, "proxied": false, "modification_time": 1605637554, "tags": [], "name": null, "commands_not_successful": 4, "consolidated_status": "complete", "selenium_version": null, "manual": false, "end_time": 1605637554, "error": null, "os": "Windows 10", "breakpointed": null, "browser": "googlechrome"}`)
			w.Write(completeStatusResp)
		case "/rest/v1/test/jobs/2":
			errorStatusResp := []byte(`{"browser_short_version": "85", "video_url": "https://localhost/jobs/2/video.mp4", "creation_time": 1605637528, "custom-data": null, "browser_version": "85.0.4183.83", "owner": "test", "automation_backend": "webdriver", "id": "2", "collects_automator_log": false, "record_screenshots": true, "record_video": true, "build": null, "passed": null, "public": "team", "assigned_tunnel_id": null, "status": "error", "log_url": "https://localhost/jobs/2/selenium-server.log", "start_time": 1605637528, "proxied": false, "modification_time": 1605637554, "tags": [], "name": null, "commands_not_successful": 4, "consolidated_status": "error", "selenium_version": null, "manual": false, "end_time": 1605637554, "error": "User Abandoned Test -- User terminated", "os": "Windows 10", "breakpointed": null, "browser": "googlechrome"}`)
			w.Write(errorStatusResp)
		case "/rest/v1/test/jobs/3":
			w.WriteHeader(http.StatusNotFound)
		case "/rest/v1/test/jobs/4":
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
			name:         "job not found error from external API",
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
			name:         "internal server error from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "333",
			expectedResp: job.Job{},
			expectedErr:  ErrServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.client.ReadJob(context.Background(), tc.jobID)
			assert.Equal(t, err, tc.expectedErr)
			assert.Equal(t, got, tc.expectedResp)
		})
	}
}

func TestClient_GetJobStatus(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/v1/test/jobs/1":
			details := &job.Job{
				ID:     "1",
				Passed: false,
				Status: "new",
				Error:  "",
			}
			randJobStatus(details, true)

			resp, _ := json.Marshal(details)
			w.Write(resp)
		case "/rest/v1/test/jobs/2":
			details := &job.Job{
				ID:     "2",
				Passed: false,
				Status: "in progress",
				Error:  "User Abandoned Test -- User terminated",
			}
			randJobStatus(details, false)

			resp, _ := json.Marshal(details)
			w.Write(resp)
		case "/rest/v1/test/jobs/3":
			w.WriteHeader(http.StatusNotFound)
		case "/rest/v1/test/jobs/4":
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
			assert.Equal(t, err, tc.expectedErr)
			assert.Equal(t, got, tc.expectedResp)
		})
	}
}

func TestClient_GetJobAssetFileNames(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/v1/test/jobs/1/assets":
			completeStatusResp := []byte(`{"console.log": "console.log", "examples__actions.spec.js.mp4": "examples__actions.spec.js.mp4", "examples__actions.spec.js.json": "examples__actions.spec.js.json", "video.mp4": "video.mp4", "selenium-log": null, "sauce-log": null, "examples__actions.spec.js.xml": "examples__actions.spec.js.xml", "video": "video.mp4", "screenshots": []}`)
			w.Write(completeStatusResp)
		case "/rest/v1/test/jobs/2/assets":
			w.WriteHeader(http.StatusNotFound)
		case "/rest/v1/test/jobs/3/assets":
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
		expectedResp []string
		expectedErr  error
	}{
		{
			name:         "get job asset with ID 1",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "1",
			expectedResp: []string{"console.log", "examples__actions.spec.js.mp4", "examples__actions.spec.js.json", "video.mp4", "examples__actions.spec.js.xml"},
			expectedErr:  nil,
		},
		{
			name:         "get job asset with ID 2",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "2",
			expectedResp: nil,
			expectedErr:  ErrJobNotFound,
		},
		{
			name:         "get job asset with ID 3",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "3",
			expectedResp: nil,
			expectedErr:  errors.New("job assets list request failed; unexpected response code:'401', msg:''"),
		},
		{
			name:         "get job asset with ID 4",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "4",
			expectedResp: nil,
			expectedErr:  ErrServerError,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.client.GetJobAssetFileNames(context.Background(), tc.jobID)
			sort.Strings(tc.expectedResp)
			sort.Strings(got)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedResp, got)
		})
	}
}

func TestClient_GetJobAssetFileContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/v1/test/jobs/1/assets/console.log":
			fileContent := []byte(`Sauce Cypress Runner 0.2.3`)
			w.Write(fileContent)
		case "/rest/v1/test/jobs/2/assets/console.log":
			w.WriteHeader(http.StatusNotFound)
		case "/rest/v1/test/jobs/3/assets/console.log":
			w.WriteHeader(http.StatusUnauthorized)
			fileContent := []byte(`unauthorized`)
			w.Write(fileContent)
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
		expectedResp []byte
		expectedErr  error
	}{
		{
			name:         "get job asset with ID 1",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "1",
			expectedResp: []byte(`Sauce Cypress Runner 0.2.3`),
			expectedErr:  nil,
		},
		{
			name:         "get job asset with ID 333 and Internal Server Error ",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "333",
			expectedResp: nil,
			expectedErr:  ErrServerError,
		},
		{
			name:         "get job asset with ID 2",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "2",
			expectedResp: nil,
			expectedErr:  ErrJobNotFound,
		},
		{
			name:         "get job asset with ID 3",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "3",
			expectedResp: nil,
			expectedErr:  errors.New("job status request failed; unexpected response code:'401', msg:'unauthorized'"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.client.GetJobAssetFileContent(context.Background(), tc.jobID, "console.log")
			assert.Equal(t, err, tc.expectedErr)
			assert.Equal(t, got, tc.expectedResp)
		})
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

func TestClient_TestStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/v1/test/jobs/1/stop":
			completeStatusResp := []byte(`{"browser_short_version": "85", "video_url": "https://localhost/jobs/1/video.mp4", "creation_time": 1605637528, "custom-data": null, "browser_version": "85.0.4183.83", "owner": "test", "automation_backend": "webdriver", "id": "1", "collects_automator_log": false, "record_screenshots": true, "record_video": true, "build": null, "passed": null, "public": "team", "assigned_tunnel_id": null, "status": "complete", "log_url": "https://localhost/jobs/1/selenium-server.log", "start_time": 1605637528, "proxied": false, "modification_time": 1605637554, "tags": [], "name": null, "commands_not_successful": 4, "consolidated_status": "complete", "selenium_version": null, "manual": false, "end_time": 1605637554, "error": null, "os": "Windows 10", "breakpointed": null, "browser": "googlechrome"}`)
			w.Write(completeStatusResp)
		case "/rest/v1/test/jobs/2/stop":
			errorStatusResp := []byte(`{"browser_short_version": "85", "video_url": "https://localhost/jobs/2/video.mp4", "creation_time": 1605637528, "custom-data": null, "browser_version": "85.0.4183.83", "owner": "test", "automation_backend": "webdriver", "id": "2", "collects_automator_log": false, "record_screenshots": true, "record_video": true, "build": null, "passed": null, "public": "team", "assigned_tunnel_id": null, "status": "error", "log_url": "https://localhost/jobs/2/selenium-server.log", "start_time": 1605637528, "proxied": false, "modification_time": 1605637554, "tags": [], "name": null, "commands_not_successful": 4, "consolidated_status": "error", "selenium_version": null, "manual": false, "end_time": 1605637554, "error": "User Abandoned Test -- User terminated", "os": "Windows 10", "breakpointed": null, "browser": "googlechrome"}`)
			w.Write(errorStatusResp)
		case "/rest/v1/test/jobs/3/stop":
			w.WriteHeader(http.StatusNotFound)
		case "/rest/v1/test/jobs/4/stop":
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
			name:         "job not found error from external API",
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
			name:         "internal server error from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "333",
			expectedResp: job.Job{},
			expectedErr:  ErrServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.client.StopJob(context.Background(), tc.jobID)
			assert.Equal(t, err, tc.expectedErr)
			assert.Equal(t, got, tc.expectedResp)
		})
	}
}

func TestClient_UploadAsset(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/testrunner/jobs/1/assets":
			w.WriteHeader(http.StatusOK)
		case "/v1/testrunner/jobs/2/assets":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("{\"uploaded\":null,\"errors\":[\"failed to upload config.yml: content-type not allowed\"]}"))
		case "/v1/testrunner/jobs/3/assets":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	timeout := 3 * time.Second

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
			name:   "Valid case",
			client: New(ts.URL, "test", "123", timeout),
			args: args{
				jobID:       "1",
				fileName:    "config.yml",
				contentType: "text/plain",
				content:     []byte("dummy-content"),
			},
			wantErr: false,
		},
		{
			name:   "invalid case - 400",
			client: New(ts.URL, "test", "123", timeout),
			args: args{
				jobID:       "2",
				fileName:    "config.yml",
				contentType: "text/plain",
				content:     []byte("dummy-content"),
			},
			wantErr: true,
		},
		{
			name:   "invalid 404",
			client: New(ts.URL, "test", "123", timeout),
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
