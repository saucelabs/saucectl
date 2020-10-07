package remotestorage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()
	storage := NewRemoteStorage()

	testCases := []struct {
		testName string
		storage  RemoteStorage
		url      string
		fileName string
		expErr   bool
		expResp  string
	}{
		{
			testName: "it should successfully upload files",
			storage:  storage,
			url:      ts.URL,
			fileName: "remotestorage.go",
			expErr:   false,
			expResp:  "Hello, client\n",
		},
		{
			testName: "it failed to upload with invalid url",
			storage:  storage,
			url:      "localhost",
			fileName: "remotestorage.go",
			expErr:   true,
		},
		{
			testName: "it failed to upload with invalid filename",
			storage:  storage,
			url:      "localhost",
			fileName: "test",
			expErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			resp, err := tc.storage.Upload(tc.url, tc.fileName, "payload")
			if err != nil {
				assert.True(t, tc.expErr)
			} else {
				assert.Equal(t, tc.expResp, string(resp))
				assert.False(t, tc.expErr)
			}
		})
	}
}
