package appstore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saucelabs/saucectl/internal/fileuploader"
	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, client")
	}))
	defer ts.Close()
	timeout := 3

	testCases := []struct {
		testName string
		upload   fileuploader.FileUploader
		fileName string
		expErr   bool
	}{
		{
			testName: "it should successfully upload files",
			upload:   New(ts.URL, "username", "access_key", timeout),
			fileName: "appstore.go",
			expErr:   false,
		},
		{
			testName: "it failed to upload with invalid url",
			upload:   New("localhost", "username", "access_key", timeout),
			fileName: "go",
			expErr:   true,
		},
		{
			testName: "it failed to upload with invalid filename",
			upload:   New(ts.URL, "username", "access_key", timeout),
			fileName: "test",
			expErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			err := tc.upload.Upload(tc.fileName, "payload")
			if err != nil {
				assert.True(t, tc.expErr)
			} else {
				assert.False(t, tc.expErr)
			}
		})
	}
}
