package appstore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saucelabs/saucectl/internal/storager"
	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, client")
	}))
	defer ts.Close()

	testCases := []struct {
		testName string
		storage  storager.Storager
		fileName string
		expErr   bool
	}{
		{
			testName: "it should successfully upload files",
			storage:  New(ts.URL, "username", "access_key"),
			fileName: "appstore.go",
			expErr:   false,
		},
		{
			testName: "it failed to upload with invalid url",
			storage:  New("localhost", "username", "access_key"),
			fileName: "go",
			expErr:   true,
		},
		{
			testName: "it failed to upload with invalid filename",
			storage:  New(ts.URL, "username", "access_key"),
			fileName: "test",
			expErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.storage.Upload(tc.fileName, "payload")
			if err != nil {
				assert.True(t, tc.expErr)
			} else {
				assert.False(t, tc.expErr)
			}
		})
	}
}
