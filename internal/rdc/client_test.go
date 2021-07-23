package rdc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/devices"
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

		client := New(ts.URL, "test", "123", timeout, config.ArtifactDownload{})
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
	client := New(ts.URL, "test-user", "test-key", timeout, config.ArtifactDownload{})

	testCases := []struct {
		name    string
		jobID   string
		want    job.Job
		wantErr error
	}{
		{
			name:    "passed job",
			jobID:   "test1",
			want:    job.Job{ID: "test1", Error: "", Status: "passed", Passed: true},
			wantErr: nil,
		},
		{
			name:    "failed job",
			jobID:   "test2",
			want:    job.Job{ID: "test2", Error: "no-device-found", Status: "failed", Passed: false},
			wantErr: nil,
		},
		{
			name:    "in progress job",
			jobID:   "test3",
			want:    job.Job{ID: "test3", Error: "", Status: "in progress", Passed: false},
			wantErr: nil,
		},
		{
			name:    "non-existant job",
			jobID:   "test4",
			want:    job.Job{ID: "test4", Error: "", Status: "", Passed: false},
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
			client: New(ts.URL, "test", "123", timeout, config.ArtifactDownload{}),
			jobID:  "1",
			expectedResp: job.Job{
				ID:     "1",
				Passed: false,
				Status: "complete",
				Error:  "",
				IsRDC:  true,
			},
			expectedErr: nil,
		},
		{
			name:   "get job details with ID 2 and status 'error'",
			client: New(ts.URL, "test", "123", timeout, config.ArtifactDownload{}),
			jobID:  "2",
			expectedResp: job.Job{
				ID:     "2",
				Passed: false,
				Status: "error",
				Error:  "User Abandoned Test -- User terminated",
				IsRDC:  true,
			},
			expectedErr: nil,
		},
		{
			name:         "user not found error from external API",
			client:       New(ts.URL, "test", "123", timeout, config.ArtifactDownload{}),
			jobID:        "3",
			expectedResp: job.Job{},
			expectedErr:  ErrJobNotFound,
		},
		{
			name:         "http status is not 200, but 401 from external API",
			client:       New(ts.URL, "test", "123", timeout, config.ArtifactDownload{}),
			jobID:        "4",
			expectedResp: job.Job{},
			expectedErr:  errors.New("job status request failed; unexpected response code:'401', msg:''"),
		},
		{
			name:         "unexpected status code from external API",
			client:       New(ts.URL, "test", "123", timeout, config.ArtifactDownload{}),
			jobID:        "333",
			expectedResp: job.Job{},
			expectedErr:  ErrServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := tc.client.PollJob(context.Background(), tc.jobID, 10*time.Millisecond, 0)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expectedResp, got)
		})
	}
}

func TestClient_GetJobAssetFileNames(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/rdc/jobs/1":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"automation_backend":"xcuitest","framework_log_url":"https://dummy/xcuitestLogs","device_log_url":"https://dummy/deviceLogs","video_url":"https://dummy/video.mp4"}`))
		case "/v1/rdc/jobs/2":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"automation_backend":"xcuitest","framework_log_url":"https://dummy/xcuitestLogs","screenshots":[{"id":"sc1"}],"video_url":"https://dummy/video.mp4"}`))
		case "/v1/rdc/jobs/3":
			w.WriteHeader(http.StatusOK)
			// The discrepancy between automation_backend and framework_log_url is wanted, as this is how the backend is currently responding.
			w.Write([]byte(`{"automation_backend":"espresso","framework_log_url":"https://dummy/xcuitestLogs","video_url":"https://dummy/video.mp4"}`))
		case "/v1/rdc/jobs/4":
			w.WriteHeader(http.StatusOK)
			// The discrepancy between automation_backend and framework_log_url is wanted, as this is how the backend is currently responding.
			w.Write([]byte(`{"automation_backend":"espresso","framework_log_url":"https://dummy/xcuitestLogs","device_log_url":"https://dummy/deviceLogs","screenshots":[{"id":"sc1"}],"video_url":"https://dummy/video.mp4"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	client := New(ts.URL, "test-user", "test-password", 1*time.Second, config.ArtifactDownload{})

	testCases := []struct {
		name     string
		jobID    string
		expected []string
		wantErr  error
	}{
		{
			name:     "XCUITest w/o screenshots",
			jobID:    "1",
			expected: []string{"device.log", "junit.xml", "video.mp4", "xcuitest.log"},
			wantErr:  nil,
		},
		{
			name:     "XCUITest w/ screenshots w/o deviceLogs",
			jobID:    "2",
			expected: []string{"junit.xml", "screenshots.zip", "video.mp4", "xcuitest.log"},
			wantErr:  nil,
		},
		{
			name:     "espresso w/o screenshots",
			jobID:    "3",
			expected: []string{"junit.xml", "video.mp4"},
			wantErr:  nil,
		},
		{
			name:     "espresso w/ screenshots w/o deviceLogs",
			jobID:    "4",
			expected: []string{"device.log", "junit.xml", "screenshots.zip", "video.mp4"},
			wantErr:  nil,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			files, err := client.GetJobAssetFileNames(context.Background(), tt.jobID)
			if err != nil {
				if !reflect.DeepEqual(err, tt.wantErr) {
					t.Errorf("GetJobAssetFileNames(): got: %v, want: %v", err, tt.wantErr)
				}
				return
			}
			if tt.wantErr != nil {
				t.Errorf("GetJobAssetFileNames(): got: %v, want: %v", err, tt.wantErr)
			}
			sort.Strings(files)
			sort.Strings(tt.expected)
			if !reflect.DeepEqual(files, tt.expected) {
				t.Errorf("GetJobAssetFileNames(): got: %v, want: %v", files, tt.expected)
			}
		})
	}
}

func TestClient_GetJobAssetFileContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/rdc/jobs/jobID/deviceLogs":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("INFO 15:10:16 1 Icing : Usage reports ok 0, Failed Usage reports 0, indexed 0, rejected 0\nINFO 15:10:16 2 GmsCoreXrpcWrapper : Returning a channel provider with trafficStatsTag=12803\nINFO 15:10:16 3 Icing : Usage reports ok 0, Failed Usage reports 0, indexed 0, rejected 0\n"))
		case "/v1/rdc/jobs/jobID/junit.xml":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<xml>junit.xml</xml>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	client := New(ts.URL, "test-user", "test-password", 1*time.Second, config.ArtifactDownload{})

	testCases := []struct {
		name     string
		jobID    string
		fileName string
		want     []byte
		wantErr  error
	}{
		{
			name:     "Download deviceLogs asset",
			jobID:    "jobID",
			fileName: "deviceLogs",
			want:     []byte("INFO 15:10:16 1 Icing : Usage reports ok 0, Failed Usage reports 0, indexed 0, rejected 0\nINFO 15:10:16 2 GmsCoreXrpcWrapper : Returning a channel provider with trafficStatsTag=12803\nINFO 15:10:16 3 Icing : Usage reports ok 0, Failed Usage reports 0, indexed 0, rejected 0\n"),
			wantErr:  nil,
		},
		{
			name:     "Download junit.xml asset",
			jobID:    "jobID",
			fileName: "junit.xml",
			want:     []byte("<xml>junit.xml</xml>"),
			wantErr:  nil,
		},
		{
			name:     "Download invalid filename",
			jobID:    "jobID",
			fileName: "buggy-file.txt",
			wantErr:  errors.New("asset not found"),
		},
	}
	for _, tt := range testCases {
		data, err := client.GetJobAssetFileContent(context.Background(), tt.jobID, tt.fileName)
		assert.Equal(t, err, tt.wantErr)
		if err == nil {
			assert.Equal(t, tt.want, data)
		}
	}
}

func TestClient_DownloadArtifact(t *testing.T) {
	fileContent := "<xml>junit.xml</xml>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/rdc/jobs/test-123":
			w.Write([]byte(`{"automation_backend":"espresso"}`))
		case "/v1/rdc/jobs/test-123/junit.xml":
			w.Write([]byte(fileContent))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	tempDir, err := os.MkdirTemp("", "saucectl-download-artifact")
	if err != nil {
		t.Errorf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	rc := New(ts.URL, "dummy-user", "dummy-key", 10*time.Second, config.ArtifactDownload{
		Directory: tempDir,
		Match:     []string{"junit.xml"},
	})
	rc.DownloadArtifact("test-123")

	fileName := filepath.Join(tempDir, "test-123", "junit.xml")
	d, err := os.ReadFile(fileName)
	if err != nil {
		t.Errorf("file '%s' not found: %v", fileName, err)
	}

	if string(d) != fileContent {
		t.Errorf("file content mismatch: got '%v', expects: '%v'", d, fileContent)
	}
}

func TestClient_GetDevices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		completeQuery := fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
		switch completeQuery {
		case "/v1/rdc/devices/filtered?os=ANDROID":
			w.Write([]byte(`{"entities":[{"name": "OnePlus 5T"},{"name": "OnePlus 6"},{"name": "OnePlus 6T"}]}`))
		case "/v1/rdc/devices/filtered?os=IOS":
			w.Write([]byte(`{"entities":[{"name": "iPhone XR"},{"name": "iPhone XS"},{"name": "iPhone X"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cl := Client{
		HTTPClient: &http.Client{Timeout: 1 * time.Second},
		URL:        ts.URL,
		Username:   "dummy-user",
		AccessKey:  "dummy-key",
	}
	type args struct {
		ctx context.Context
		OS  string
	}
	tests := []struct {
		name    string
		args    args
		want    []devices.Device
		wantErr bool
	}{
		{
			name: "Android devices",
			args: args{
				ctx: context.Background(),
				OS:  "ANDROID",
			},
			want: []devices.Device{
				{Name: "OnePlus 5T"},
				{Name: "OnePlus 6"},
				{Name: "OnePlus 6T"},
			},
			wantErr: false,
		},
		{
			name: "iOS devices",
			args: args{
				ctx: context.Background(),
				OS:  "IOS",
			},
			want: []devices.Device{
				{Name: "iPhone XR"},
				{Name: "iPhone XS"},
				{Name: "iPhone X"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cl.GetDevices(tt.args.ctx, tt.args.OS)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDevices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDevices() got = %v, want %v", got, tt.want)
			}
		})
	}
}
