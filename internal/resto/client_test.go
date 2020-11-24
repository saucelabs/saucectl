package resto

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClient_GetJobDetails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/v1/test/jobs/1":
			completeStatusResp := []byte(`{"browser_short_version": "85", "video_url": "https://localhost/jobs/1/video.mp4", "creation_time": 1605637528, "custom-data": null, "browser_version": "85.0.4183.83", "owner": "test", "automation_backend": "webdriver", "id": "1", "collects_automator_log": false, "record_screenshots": true, "record_video": true, "build": null, "passed": null, "public": "team", "assigned_tunnel_id": null, "status": "complete", "log_url": "https://localhost/jobs/1/selenium-server.log", "start_time": 1605637528, "proxied": false, "modification_time": 1605637554, "tags": [], "name": null, "commands_not_successful": 4, "consolidated_status": "complete", "selenium_version": null, "manual": false, "end_time": 1605637554, "error": null, "os": "Windows 10", "breakpointed": null, "browser": "googlechrome"}`)
			w.Write(completeStatusResp)
		case "/rest/v1/test/jobs/2":
			errorStatusResp := []byte(`{"browser_short_version": "85", "video_url": "https://localhost/jobs/2/video.mp4", "creation_time": 1605637528, "custom-data": null, "browser_version": "85.0.4183.83", "owner": "test", "automation_backend": "webdriver", "id": "2", "collects_automator_log": false, "record_screenshots": true, "record_video": true, "build": null, "passed": null, "public": "team", "assigned_tunnel_id": null, "status": "error", "log_url": "https://localhost/jobs/2/selenium-server.log", "start_time": 1605637528, "proxied": false, "modification_time": 1605637554, "tags": [], "name": null, "commands_not_successful": 4, "consolidated_status": "error", "selenium_version": null, "manual": false, "end_time": 1605637554, "error": null, "os": "Windows 10", "breakpointed": null, "browser": "googlechrome"}`)
			w.Write(errorStatusResp)
		case "/rest/v1/test/jobs/3":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	timeout := 3

	testCases := []struct{
		name string
		client Client
		jobID string
		expectedResp Details
		expectedErr error
	}{
		{
			name: "get job details with ID 1 and status 'complete'",
			client: New(ts.URL, "test", "123", timeout),
			jobID: "1",
			expectedResp: Details{
				BrowserShortVersion:"85",
				VideoURL:"https://localhost/jobs/1/video.mp4",
				CreationTime:1605637528,
				CustomData:nil,
				BrowserVersion:"85.0.4183.83",
				Owner:"test",
				AutomationBackend:"webdriver",
				ID:"1",
				CollectsAutomatorLog:false,
				RecordScreenshots:true,
				RecordVideo:true,
				Build:nil,
				Passed:nil,
				Public:nil,
				AssignedTunnelID:nil,
				Status:"complete",
				LogURL:"https://localhost/jobs/1/selenium-server.log",
				StartTime:1605637528,
				Proxied:false,
				ModificationTime:1605637554,
				Tags:[]string{},
				Name:nil,
				CommandsNotSuccessful:4,
				ConsolidatedStatus:"complete",
				SeleniumVersion:nil,
				Manual:false,
				EndTime:1605637554,
				Error:nil,
				OS:"Windows 10",
				Breakpointed:nil,
				Browser:"googlechrome",
			},
			expectedErr: nil,
		},
		{
			name: "get job details with ID 2 and status 'error'",
			client: New(ts.URL, "test", "123", timeout),
			jobID: "2",
			expectedResp: Details{
				BrowserShortVersion:"85",
				VideoURL:"https://localhost/jobs/2/video.mp4",
				CreationTime:1605637528,
				CustomData:nil,
				BrowserVersion:"85.0.4183.83",
				Owner:"test",
				AutomationBackend:"webdriver",
				ID:"2",
				CollectsAutomatorLog:false,
				RecordScreenshots:true,
				RecordVideo:true,
				Build:nil,
				Passed:nil,
				Public:nil,
				AssignedTunnelID:nil,
				Status:"error",
				LogURL:"https://localhost/jobs/2/selenium-server.log",
				StartTime:1605637528,
				Proxied:false,
				ModificationTime:1605637554,
				Tags:[]string{},
				Name:nil,
				CommandsNotSuccessful:4,
				ConsolidatedStatus:"error",
				SeleniumVersion:nil,
				Manual:false,
				EndTime:1605637554,
				Error:nil,
				OS:"Windows 10",
				Breakpointed:nil,
				Browser:"googlechrome",
			},
			expectedErr: nil,
		},
		{
			name:         "user not found error from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "3",
			expectedResp: Details{},
			expectedErr:  ErrNotFoundUser,
		},
		{
			name:         "internal server error from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "333",
			expectedResp: Details{},
			expectedErr:  ErrServerInaccessible,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.client.GetJobDetails(tc.jobID)
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
			details := &Details{
				BrowserShortVersion:"85",
				VideoURL:"https://localhost/jobs/1/video.mp4",
				CreationTime:1605637528,
				CustomData:nil,
				BrowserVersion:"85.0.4183.83",
				Owner:"test",
				AutomationBackend:"webdriver",
				ID:"1",
				CollectsAutomatorLog:false,
				RecordScreenshots:true,
				RecordVideo:true,
				Build:nil,
				Passed:nil,
				Public:nil,
				AssignedTunnelID:nil,
				Status:"new",
				LogURL:"https://localhost/jobs/1/selenium-server.log",
				StartTime:1605637528,
				Proxied:false,
				ModificationTime:1605637554,
				Tags:[]string{},
				Name:nil,
				CommandsNotSuccessful:4,
				ConsolidatedStatus:"new",
				SeleniumVersion:nil,
				Manual:false,
				EndTime:1605637554,
				Error:nil,
				OS:"Windows 10",
				Breakpointed:nil,
				Browser:"googlechrome",
			}
			randJobStatus(details, true)

			resp, _ := json.Marshal(details)
			w.Write(resp)
		case "/rest/v1/test/jobs/2":
			details := &Details{
				BrowserShortVersion:"85",
				VideoURL:"https://localhost/jobs/2/video.mp4",
				CreationTime:1605637528,
				CustomData:nil,
				BrowserVersion:"85.0.4183.83",
				Owner:"test",
				AutomationBackend:"webdriver",
				ID:"2",
				CollectsAutomatorLog:false,
				RecordScreenshots:true,
				RecordVideo:true,
				Build:nil,
				Passed:nil,
				Public:nil,
				AssignedTunnelID:nil,
				Status:"in progress",
				LogURL:"https://localhost/jobs/2/selenium-server.log",
				StartTime:1605637528,
				Proxied:false,
				ModificationTime:1605637554,
				Tags:[]string{},
				Name:nil,
				CommandsNotSuccessful:4,
				ConsolidatedStatus:"in progress",
				SeleniumVersion:nil,
				Manual:false,
				EndTime:1605637554,
				Error:nil,
				OS:"Windows 10",
				Breakpointed:nil,
				Browser:"googlechrome",
			}
			randJobStatus(details, false)

			resp, _ := json.Marshal(details)
			w.Write(resp)
		case "/rest/v1/test/jobs/3":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	timeout := 3

	testCases := []struct{
		name string
		client Client
		jobID string
		expectedResp Details
		expectedErr error
	}{
		{
			name: "get job details with ID 1 and status 'complete'",
			client: New(ts.URL, "test", "123", timeout),
			jobID: "1",
			expectedResp: Details{
				BrowserShortVersion:"85",
				VideoURL:"https://localhost/jobs/1/video.mp4",
				CreationTime:1605637528,
				CustomData:nil,
				BrowserVersion:"85.0.4183.83",
				Owner:"test",
				AutomationBackend:"webdriver",
				ID:"1",
				CollectsAutomatorLog:false,
				RecordScreenshots:true,
				RecordVideo:true,
				Build:nil,
				Passed:nil,
				Public:nil,
				AssignedTunnelID:nil,
				Status:"complete",
				LogURL:"https://localhost/jobs/1/selenium-server.log",
				StartTime:1605637528,
				Proxied:false,
				ModificationTime:1605637554,
				Tags:[]string{},
				Name:nil,
				CommandsNotSuccessful:4,
				ConsolidatedStatus:"complete",
				SeleniumVersion:nil,
				Manual:false,
				EndTime:1605637554,
				Error:nil,
				OS:"Windows 10",
				Breakpointed:nil,
				Browser:"googlechrome",
			},
			expectedErr: nil,
		},
		{
			name: "get job details with ID 2 and status 'error'",
			client: New(ts.URL, "test", "123", timeout),
			jobID: "2",
			expectedResp: Details{
				BrowserShortVersion:"85",
				VideoURL:"https://localhost/jobs/2/video.mp4",
				CreationTime:1605637528,
				CustomData:nil,
				BrowserVersion:"85.0.4183.83",
				Owner:"test",
				AutomationBackend:"webdriver",
				ID:"2",
				CollectsAutomatorLog:false,
				RecordScreenshots:true,
				RecordVideo:true,
				Build:nil,
				Passed:nil,
				Public:nil,
				AssignedTunnelID:nil,
				Status:"error",
				LogURL:"https://localhost/jobs/2/selenium-server.log",
				StartTime:1605637528,
				Proxied:false,
				ModificationTime:1605637554,
				Tags:[]string{},
				Name:nil,
				CommandsNotSuccessful:4,
				ConsolidatedStatus:"error",
				SeleniumVersion:nil,
				Manual:false,
				EndTime:1605637554,
				Error:nil,
				OS:"Windows 10",
				Breakpointed:nil,
				Browser:"googlechrome",
			},
			expectedErr: nil,
		},
		{
			name:         "user not found error from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "3",
			expectedResp: Details{},
			expectedErr:  ErrNotFoundUser,
		},
		{
			name:         "unexpected status code from external API",
			client:       New(ts.URL, "test", "123", timeout),
			jobID:        "333",
			expectedResp: Details{},
			expectedErr:  ErrServerInaccessible,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.client.GetJobStatus(tc.jobID, 10 * time.Millisecond)
			assert.Equal(t, err, tc.expectedErr)
			assert.Equal(t, got, tc.expectedResp)
		})
	}
}

func randJobStatus(details *Details, isComplete bool) {
	min := 1
	max := 10
	randNum := rand.Intn(max - min + 1) + min

	status := "error"
	if isComplete {
		status = "complete"
	}

	if randNum >= 5 {
		details.Status = status
		details.ConsolidatedStatus = status
	}
}