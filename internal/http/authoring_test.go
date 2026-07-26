package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/stretchr/testify/assert"
)

const (
	testUser = "test-user"
	testPass = "test-pass"
)

func checkAuth(t *testing.T, r *http.Request) bool {
	t.Helper()
	user, pass, ok := r.BasicAuth()
	return ok && user == testUser && pass == testPass
}

func TestAuthoringService_GenerateAndPollTestCase(t *testing.T) {
	taskID := "task-123"
	testCaseID := "64f0a1b2c3d4e5f6a7b8c9d0"
	pollCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(t, r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/ai-authoring/v1/testcases/generate":
			var body map[string]interface{}
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "login test", body["name"])

			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]string{"taskId": taskID, "sauceJobId": "job-1"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ai-authoring/v1/testcases/generate/"+taskID:
			pollCount++
			if pollCount < 2 {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{"status": "PENDING"},
				})
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"status":     "COMPLETED",
					"testCaseId": testCaseID,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	svc := NewAuthoringService(server.URL, testUser, testPass, 10*time.Second)

	task, err := svc.GenerateTestCase(context.Background(), authoring.GenerateRequest{
		Name: "login test",
		PromptSettings: authoring.PromptSettings{
			Intent: "go to the login page and sign in",
		},
		RunSettings: authoring.RunSettings{
			Target: map[string]interface{}{"capabilities": map[string]interface{}{"browserName": "chrome"}},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, taskID, task.TaskID)
	assert.Equal(t, authoring.TaskPending, task.Status)

	// First poll: still pending.
	task, err = svc.GetGenerateTask(context.Background(), taskID)
	assert.NoError(t, err)
	assert.Equal(t, authoring.TaskPending, task.Status)

	// Second poll: completed.
	task, err = svc.GetGenerateTask(context.Background(), taskID)
	assert.NoError(t, err)
	assert.Equal(t, authoring.TaskCompleted, task.Status)
	assert.Equal(t, testCaseID, task.TestCaseID)
}

func TestAuthoringService_GenerateTestCase_WrongCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	svc := NewAuthoringService(server.URL, "wrong", "creds", 10*time.Second)
	_, err := svc.GenerateTestCase(context.Background(), authoring.GenerateRequest{Name: "x"})
	assert.Error(t, err)
}

func TestAuthoringService_DeleteTestCase_NotFoundIsNoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	svc := NewAuthoringService(server.URL, testUser, testPass, 10*time.Second)
	assert.NoError(t, svc.DeleteTestCase(context.Background(), "some-id"))
}

func TestAuthoringService_CreateAndUpdateTestSuite(t *testing.T) {
	suiteID := "11111111-1111-1111-1111-111111111111"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(t, r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/ai-authoring/v1/testsuites":
			var body map[string]interface{}
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "regression", body["name"])

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{"id": suiteID, "name": "regression", "testCaseCount": 0},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/ai-authoring/v1/testsuites/"+suiteID:
			var body map[string]interface{}
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			addIDs, _ := body["addTestCases"].([]interface{})
			assert.Len(t, addIDs, 1)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{"id": suiteID, "name": "regression", "testCaseCount": 1},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/ai-authoring/v1/testsuites":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"items": []map[string]interface{}{
						{"id": suiteID, "name": "regression", "testCaseCount": 1},
					},
					"total": 1,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	svc := NewAuthoringService(server.URL, testUser, testPass, 10*time.Second)

	suite, err := svc.CreateTestSuite(context.Background(), "regression", nil)
	assert.NoError(t, err)
	assert.Equal(t, suiteID, suite.ID)

	suite, err = svc.UpdateTestSuite(context.Background(), suiteID, authoring.UpdateTestSuiteOptions{
		AddTestCases: []string{"64f0a1b2c3d4e5f6a7b8c9d0"},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, suite.TestCaseCount)

	suites, err := svc.ListTestSuites(context.Background(), authoring.ListTestSuiteOptions{Search: "regression"})
	assert.NoError(t, err)
	assert.Len(t, suites, 1)
	assert.Equal(t, suiteID, suites[0].ID)
}

func TestAuthoringService_ListTestCases(t *testing.T) {
	suiteID := "33333333-3333-3333-3333-333333333333"
	tc1 := "64f0a1b2c3d4e5f6a7b8c9d0"
	tc2 := "64f0a1b2c3d4e5f6a7b8c9d1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(t, r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path != "/ai-authoring/v1/testcases" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assert.Equal(t, suiteID, r.URL.Query().Get("testSuiteId"))

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": tc1, "name": "login", "testSuiteId": suiteID},
					{"id": tc2, "name": "checkout", "testSuiteId": suiteID},
				},
				"total": 2,
			},
		})
	}))
	defer server.Close()

	svc := NewAuthoringService(server.URL, testUser, testPass, 10*time.Second)
	cases, total, err := svc.ListTestCases(context.Background(), authoring.ListTestCaseOptions{TestSuiteID: suiteID})
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, cases, 2)
	assert.Equal(t, tc1, cases[0].ID)
	assert.Equal(t, tc2, cases[1].ID)
}

func TestAuthoringService_RunTestCase(t *testing.T) {
	testCaseID := "64f0a1b2c3d4e5f6a7b8c9d0"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ai-authoring/v1/testcases/"+testCaseID+"/run" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]interface{}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "pr-123-abcdef", body["buildName"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":         "run-1",
				"testCaseId": testCaseID,
				"build":      "pr-123-abcdef",
				"jobs": []map[string]interface{}{
					{
						"id":         "job-a",
						"sauceJobId": "sauce-job-1",
						"target":     "chrome",
						"name":       "login - chrome",
						"url":        "https://app.saucelabs.com/tests/sauce-job-1",
						"isRdc":      false,
						"success":    false,
					},
				},
			},
		})
	}))
	defer server.Close()

	svc := NewAuthoringService(server.URL, testUser, testPass, 10*time.Second)
	run, err := svc.RunTestCase(context.Background(), testCaseID, authoring.RunTestCaseOptions{BuildName: "pr-123-abcdef"})
	assert.NoError(t, err)
	assert.Equal(t, testCaseID, run.TestCaseID)
	assert.Len(t, run.Jobs, 1)
	assert.Equal(t, "sauce-job-1", run.Jobs[0].SauceJobID)
	assert.False(t, run.Jobs[0].IsRDC)
}

func TestAuthoringService_RunTestSuite(t *testing.T) {
	suiteID := "22222222-2222-2222-2222-222222222222"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ai-authoring/v1/testsuites/"+suiteID+"/run" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]interface{}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "pr-123-abcdef", body["buildName"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id": "run-1", "runCount": 5, "buildName": "pr-123-abcdef",
			},
		})
	}))
	defer server.Close()

	svc := NewAuthoringService(server.URL, testUser, testPass, 10*time.Second)
	run, err := svc.RunTestSuite(context.Background(), suiteID, "pr-123-abcdef")
	assert.NoError(t, err)
	assert.Equal(t, 5, run.RunCount)
	assert.Equal(t, "pr-123-abcdef", run.BuildName)
}
