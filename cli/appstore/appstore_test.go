package appstore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	validResp := `{"item": {"id": "23bb197a-440f-4969-aa00-8229921e0927", "owner": {"id": "044cd69cd08c4687bb3b0ea63a924ddc", "org_id": "b7bf895c370a4ca1ac1a1dea688a108a"}, "name": "test.zip", "upload_timestamp": 1602199902, "etag": "1ceca21e2de7c8b9bbe09a45fce1168b", "kind": "other", "group_id": 60841, "metadata": null, "access": {"team_ids": ["05ded95b41b441b480b74282b8f97a40"], "org_ids": []}}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, validResp)
	}))
	defer ts.Close()

	testCases := []struct {
		testName string
		storage  AppStore
		fileName string
		expErr   bool
		expResp  string
	}{
		{
			testName: "it should successfully upload files",
			storage:  New(ts.URL),
			fileName: "appstore.go",
			expErr:   false,
			expResp:  "Hello, client\n",
		},
		{
			testName: "it failed to upload with invalid url",
			storage:  New("localhost"),
			fileName: "go",
			expErr:   true,
		},
		{
			testName: "it failed to upload with invalid filename",
			storage:  New(ts.URL),
			fileName: "test",
			expErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			resp, err := tc.storage.Upload(tc.fileName, "payload")
			if err != nil {
				assert.True(t, tc.expErr)
			} else {
				assert.NotEmpty(t, resp.Item.ID)
				assert.False(t, tc.expErr)
			}
		})
	}
}
