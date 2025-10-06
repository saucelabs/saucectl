package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/retry"

	"github.com/google/go-cmp/cmp"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/devices/devicestatus"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/stretchr/testify/assert"
)

func TestRDCService_ReadJob(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/rdc/jobs/test1":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{"id": "test1", "error": null, "status": "passed", "consolidated_status": "passed"}`))
		case "/v1/rdc/jobs/test2":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{"id": "test2", "error": "no-device-found", "status": "failed", "consolidated_status": "failed"}`))
		case "/v1/rdc/jobs/test3":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{"id": "test3", "error": null, "status": "in progress", "consolidated_status": "in progress"}`))
		case "/v1/rdc/jobs/test4":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	timeout := 3 * time.Second
	client := NewRDCService(region.None, "test-user", "test-key", timeout)
	client.URL = ts.URL
	client.Client.RetryMax = 0

	testCases := []struct {
		name    string
		jobID   string
		want    job.Job
		wantErr error
	}{
		{
			name:  "passed job",
			jobID: "test1",
			want: job.Job{
				ID:     "test1",
				Error:  "",
				Status: "passed",
				Passed: true,
				IsRDC:  true,
				URL:    "/tests/test1",
			},
			wantErr: nil,
		},
		{
			name:  "failed job",
			jobID: "test2",
			want: job.Job{
				ID:     "test2",
				Error:  "no-device-found",
				Status: "failed",
				Passed: false,
				IsRDC:  true,
				URL:    "/tests/test2",
			},
			wantErr: nil,
		},
		{
			name:  "in progress job",
			jobID: "test3",
			want: job.Job{
				ID:     "test3",
				Error:  "",
				Status: "in progress",
				Passed: false,
				IsRDC:  true,
				URL:    "/tests/test3",
			},
			wantErr: nil,
		},
		{
			name:  "non-existent job",
			jobID: "test4",
			want: job.Job{
				ID:     "test4",
				Error:  "",
				Status: "",
				Passed: false,
				URL:    "/tests/test4",
			},
			wantErr: ErrJobNotFound,
		},
	}

	for _, tt := range testCases {
		t.Run(
			tt.name, func(t *testing.T) {
				j, err := client.Job(context.Background(), tt.jobID, true)
				assert.Equal(t, err, tt.wantErr)
				if err == nil {
					assert.Equal(t, tt.want, j)
				}
			},
		)
	}
}

func TestRDCService_PollJob(t *testing.T) {
	var retryCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/rdc/jobs/1":
			_ = json.NewEncoder(w).Encode(rdcJob{
				ID:     "1",
				Status: job.StateComplete,
			})
		case "/v1/rdc/jobs/2":
			_ = json.NewEncoder(w).Encode(rdcJob{
				ID:     "2",
				Passed: false,
				Status: job.StateError,
				Error:  "User Abandoned Test -- User terminated",
			})
		case "/v1/rdc/jobs/3":
			w.WriteHeader(http.StatusNotFound)
		case "/v1/rdc/jobs/4":
			w.WriteHeader(http.StatusUnauthorized)
		case "/v1/rdc/jobs/5":
			if retryCount < 2 {
				w.WriteHeader(http.StatusInternalServerError)
				retryCount++
				return
			}

			_ = json.NewEncoder(w).Encode(rdcJob{
				ID:     "5",
				Status: job.StatePassed,
				Passed: true,
				Error:  "",
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	timeout := 3 * time.Second

	rdc := NewRDCService(region.None, "test", "123", timeout)
	rdc.URL = ts.URL

	testCases := []struct {
		name         string
		client       RDCService
		jobID        string
		expectedResp job.Job
		expectedErr  error
	}{
		{
			name:   "get job details with ID 1 and status 'complete'",
			client: rdc,
			jobID:  "1",
			expectedResp: job.Job{
				ID:     "1",
				Passed: true,
				Status: "complete",
				Error:  "",
				IsRDC:  true,
				URL:    "/tests/1",
			},
			expectedErr: nil,
		},
		{
			name:   "get job details with ID 2 and status 'error'",
			client: rdc,
			jobID:  "2",
			expectedResp: job.Job{
				ID:     "2",
				Passed: false,
				Status: "error",
				Error:  "User Abandoned Test -- User terminated",
				IsRDC:  true,
				URL:    "/tests/2",
			},
			expectedErr: nil,
		},
		{
			name:         "job not found error from external API",
			client:       rdc,
			jobID:        "3",
			expectedResp: job.Job{ID: "3"},
			expectedErr:  ErrJobNotFound,
		},
		{
			name:         "http status is not 200, but 401 from external API",
			client:       rdc,
			jobID:        "4",
			expectedResp: job.Job{ID: "4"},
			expectedErr:  errors.New("unexpected statusCode: 401"),
		},
		{
			name:         "unexpected status code from external API",
			client:       rdc,
			jobID:        "333",
			expectedResp: job.Job{ID: "333"},
			expectedErr:  errors.New("internal server error"),
		},
		{
			name:   "get job details with ID 5. retry 2 times and succeed",
			client: rdc,
			jobID:  "5",
			expectedResp: job.Job{
				ID:     "5",
				Passed: true,
				Status: job.StatePassed,
				Error:  "",
				IsRDC:  true,
				URL:    "/tests/5",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.client.Client.RetryWaitMax = 1 * time.Millisecond
			got, err := tc.client.PollJob(context.Background(), tc.jobID, 10*time.Millisecond, 0, true)
			assert.Equal(t, tc.expectedResp, got)
			if err != nil {
				assert.Equal(t, tc.expectedErr, err)
			}
		})
	}
}

func TestRDCService_GetJobAssetFileNames(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/rdc/jobs/1":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{"automation_backend":"xcuitest","framework_log_url":"https://dummy/xcuitestLogs","device_log_url":"https://dummy/deviceLogs","video_url":"https://dummy/video.mp4"}`))
		case "/v1/rdc/jobs/2":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte(`{"automation_backend":"xcuitest","framework_log_url":"https://dummy/xcuitestLogs","screenshots":[{"id":"sc1"}],"video_url":"https://dummy/video.mp4"}`))
		case "/v1/rdc/jobs/3":
			w.WriteHeader(http.StatusOK)
			// The discrepancy between automation_backend and framework_log_url is wanted, as this is how the backend is currently responding.
			_, err = w.Write([]byte(`{"automation_backend":"espresso","framework_log_url":"https://dummy/xcuitestLogs","video_url":"https://dummy/video.mp4"}`))
		case "/v1/rdc/jobs/4":
			w.WriteHeader(http.StatusOK)
			// The discrepancy between automation_backend and framework_log_url is wanted, as this is how the backend is currently responding.
			_, err = w.Write([]byte(`{"automation_backend":"espresso","framework_log_url":"https://dummy/xcuitestLogs","device_log_url":"https://dummy/deviceLogs","screenshots":[{"id":"sc1"}],"video_url":"https://dummy/video.mp4"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	client := NewRDCService(region.None, "test-user", "test-password", 1*time.Second)
	client.URL = ts.URL

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
			files, err := client.ArtifactNames(context.Background(), tt.jobID, true)
			if err != nil {
				if !reflect.DeepEqual(err, tt.wantErr) {
					t.Errorf("ArtifactNames(): got: %v, want: %v", err, tt.wantErr)
				}
				return
			}
			if tt.wantErr != nil {
				t.Errorf("ArtifactNames(): got: %v, want: %v", err, tt.wantErr)
			}
			sort.Strings(files)
			sort.Strings(tt.expected)
			if !reflect.DeepEqual(files, tt.expected) {
				t.Errorf("ArtifactNames(): got: %v, want: %v", files, tt.expected)
			}
		})
	}
}

func TestRDCService_GetJobAssetFileContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1/rdc/jobs/jobID/deviceLogs":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("INFO 15:10:16 1 Icing : Usage reports ok 0, Failed Usage reports 0, indexed 0, rejected 0\nINFO 15:10:16 2 GmsCoreXrpcWrapper : Returning a channel provider with trafficStatsTag=12803\nINFO 15:10:16 3 Icing : Usage reports ok 0, Failed Usage reports 0, indexed 0, rejected 0\n"))
		case "/v1/rdc/jobs/jobID/junit.xml":
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("<xml>junit.xml</xml>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	client := NewRDCService(region.None, "test-user", "test-password", 1*time.Second)
	client.URL = ts.URL
	client.Client.RetryMax = 0

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
		data, err := client.Artifact(context.Background(), tt.jobID, tt.fileName, true, retry.CreateOptions())
		assert.Equal(t, err, tt.wantErr)
		if err == nil {
			assert.Equal(t, tt.want, data)
		}
	}
}

func TestRDCService_GetDevicesByOS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		completeQuery := fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
		switch completeQuery {
		case "/v1/rdc/devices/filtered?os=ANDROID":
			_, err = w.Write([]byte(`{"entities":[
				{"name": "OnePlus 5T", "os": "ANDROID", "osVersion": "10.0"},
				{"name": "OnePlus 6", "os": "ANDROID", "osVersion": "10.0"},
				{"name": "OnePlus 6T", "os": "ANDROID", "osVersion": "10.0"}
			]}`))
		case "/v1/rdc/devices/filtered?os=IOS":
			_, err = w.Write([]byte(`{"entities":[
				{"name": "iPhone XR", "os": "IOS", "osVersion": "10.0"},
				{"name": "iPhone XS", "os": "IOS", "osVersion": "10.0"},
				{"name": "iPhone X", "os": "IOS", "osVersion": "10.0"}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	client := retryablehttp.NewClient()
	client.HTTPClient = &http.Client{Timeout: 1 * time.Second}

	cl := RDCService{
		Client:    client,
		URL:       ts.URL,
		Username:  "dummy-user",
		AccessKey: "dummy-key",
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
				{Name: "OnePlus 5T", OS: "ANDROID", OSVersion: "10.0"},
				{Name: "OnePlus 6", OS: "ANDROID", OSVersion: "10.0"},
				{Name: "OnePlus 6T", OS: "ANDROID", OSVersion: "10.0"},
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
				{Name: "iPhone XR", OS: "IOS", OSVersion: "10.0"},
				{Name: "iPhone XS", OS: "IOS", OSVersion: "10.0"},
				{Name: "iPhone X", OS: "IOS", OSVersion: "10.0"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cl.GetDevicesByOS(tt.args.ctx, tt.args.OS)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDevicesByOS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDevicesByOS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRDCService_GetDevices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var err error
		_, err = w.Write([]byte(`[
			{"name": "OnePlus 5T", "os": "ANDROID", "osVersion": "10.0"},
			{"name": "OnePlus 6", "os": "ANDROID", "osVersion": "10.0"},
			{"name": "OnePlus 6T", "os": "ANDROID", "osVersion": "10.0"},
			{"name": "iPhone XR", "os": "IOS", "osVersion": "10.0"},
			{"name": "iPhone XS", "os": "IOS", "osVersion": "10.0"},
			{"name": "iPhone X", "os": "IOS", "osVersion": "10.0"}
		]`))
		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	client := retryablehttp.NewClient()
	client.HTTPClient = &http.Client{Timeout: 1 * time.Second}

	cl := RDCService{
		Client:    client,
		URL:       ts.URL,
		Username:  "dummy-user",
		AccessKey: "dummy-key",
	}

	ctx := context.Background()
	want := []devices.Device{
		{Name: "OnePlus 5T", OS: "ANDROID", OSVersion: "10.0"},
		{Name: "OnePlus 6", OS: "ANDROID", OSVersion: "10.0"},
		{Name: "OnePlus 6T", OS: "ANDROID", OSVersion: "10.0"},
		{Name: "iPhone XR", OS: "IOS", OSVersion: "10.0"},
		{Name: "iPhone XS", OS: "IOS", OSVersion: "10.0"},
		{Name: "iPhone X", OS: "IOS", OSVersion: "10.0"},
	}

	got, err := cl.GetDevices(ctx)
	if err != nil {
		t.Errorf("GetDevices() error = %v", err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetDevices() got = %v, want %v", got, want)
	}
}

func TestRDCService_GetDevicesStatuses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var err error
		_, err = w.Write([]byte(`{"devices":[
			{"descriptor": "OnePlus 5T","state":"AVAILABLE"},
			{"descriptor": "OnePlus 6","state":"IN_USE"},
			{"descriptor": "OnePlus 6T","state":"CLEANING"},
			{"descriptor": "iPhone XR","state":"MAINTENANCE"},
			{"descriptor": "iPhone XS","state":"REBOOTING"},
			{"descriptor": "iPhone X","state":"OFFLINE"}
		]}`))
		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	client := retryablehttp.NewClient()
	client.HTTPClient = &http.Client{Timeout: 1 * time.Second}

	cl := RDCService{
		Client:    client,
		URL:       ts.URL,
		Username:  "dummy-user",
		AccessKey: "dummy-key",
	}

	ctx := context.Background()
	want := []devices.DeviceStatus{
		{ID: "OnePlus 5T", Status: devicestatus.Available},
		{ID: "OnePlus 6", Status: devicestatus.InUse},
		{ID: "OnePlus 6T", Status: devicestatus.Cleaning},
		{ID: "iPhone XR", Status: devicestatus.Maintenance},
		{ID: "iPhone XS", Status: devicestatus.Rebooting},
		{ID: "iPhone X", Status: devicestatus.Offline},
	}

	got, err := cl.GetDevicesStatuses(ctx)
	if err != nil {
		t.Errorf("GetDevicesStatuses() error = %v", err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetDevicesStatuses() got = %v, want %v", got, want)
	}
}

func TestRDCService_parseJob(t *testing.T) {
	client := NewRDCService(region.None, "test-user", "test-key", 3*time.Second)
	client.AppURL = "https://app.saucelabs.com"

	tests := []struct {
		name    string
		input   string
		want    job.Job
		wantErr bool
	}{
		{
			name:  "parse job with complete status",
			input: `{"id": "test1", "name": "Test Job", "status": "complete", "automation_backend": "xcuitest", "os": "iOS", "os_version": "15.0", "device_name": "iPhone 12"}`,
			want: job.Job{
				ID:         "test1",
				Name:       "Test Job",
				Status:     "complete",
				Passed:     true, // Should be true for complete status
				DeviceName: "iPhone 12",
				Framework:  "xcuitest",
				OS:         "iOS",
				OSVersion:  "15.0",
				IsRDC:      true,
				URL:        "https://app.saucelabs.com/tests/test1",
			},
			wantErr: false,
		},
		{
			name:  "parse job with passed status",
			input: `{"id": "test2", "name": "Test Job 2", "status": "passed", "automation_backend": "espresso", "os": "Android", "os_version": "11.0", "device_name": "Pixel 5"}`,
			want: job.Job{
				ID:         "test2",
				Name:       "Test Job 2",
				Status:     "passed",
				Passed:     true, // Should be true for passed status
				DeviceName: "Pixel 5",
				Framework:  "espresso",
				OS:         "Android",
				OSVersion:  "11.0",
				IsRDC:      true,
				URL:        "https://app.saucelabs.com/tests/test2",
			},
			wantErr: false,
		},
		{
			name:  "parse job with failed status",
			input: `{"id": "test3", "name": "Test Job 3", "status": "failed", "error": "Test failed", "automation_backend": "xcuitest", "os": "iOS", "os_version": "14.0", "device_name": "iPhone 11"}`,
			want: job.Job{
				ID:         "test3",
				Name:       "Test Job 3",
				Status:     "failed",
				Passed:     false, // Should be false for failed status
				Error:      "Test failed",
				DeviceName: "iPhone 11",
				Framework:  "xcuitest",
				OS:         "iOS",
				OSVersion:  "14.0",
				IsRDC:      true,
				URL:        "https://app.saucelabs.com/tests/test3",
			},
			wantErr: false,
		},
		{
			name:  "parse job with in progress status",
			input: `{"id": "test4", "name": "Test Job 4", "status": "in progress", "automation_backend": "espresso", "os": "Android", "os_version": "12.0", "device_name": "Samsung Galaxy S21"}`,
			want: job.Job{
				ID:         "test4",
				Name:       "Test Job 4",
				Status:     "in progress",
				Passed:     false, // Should be false for in progress status
				DeviceName: "Samsung Galaxy S21",
				Framework:  "espresso",
				OS:         "Android",
				OSVersion:  "12.0",
				IsRDC:      true,
				URL:        "https://app.saucelabs.com/tests/test4",
			},
			wantErr: false,
		},
		{
			name:    "parse invalid JSON",
			input:   `{"id": "test5", "name": "Test Job 5", "status": "complete", invalid json}`,
			want:    job.Job{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := io.NopCloser(strings.NewReader(tt.input))
			got, err := client.parseJob(body)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("parseJob() (-want +got): \n%s", diff)
				}
			}
		})
	}
}

func TestRDCService_StartJob(t *testing.T) {
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
				ctx: context.Background(),
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
				Status: job.StateQueued,
				IsRDC:  true,
				URL:    "/tests/fake-job-id",
			},
			wantErr: nil,
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(201)
				_, _ = w.Write([]byte(`{ "test_report": { "id": "fake-job-id" }}`))
			},
		},
		{
			name: "Non 2xx status code",
			args: args{
				ctx:               context.Background(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    job.Job{},
			wantErr: fmt.Errorf("job start failed; unexpected response code:'300', msg:''"),
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(300)
			},
		},
		{
			name: "Unknown error",
			args: args{
				ctx:               context.Background(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    job.Job{},
			wantErr: fmt.Errorf("job start failed; unexpected response code:'500', msg:'Internal server error'"),
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
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

			c := &RDCService{
				Client: &retryablehttp.Client{HTTPClient: server.Client()},
				URL:    server.URL,
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
