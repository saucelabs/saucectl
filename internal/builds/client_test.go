package builds

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
)

func TestClient_GetBuildForJob(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		responseBody []byte
		want         string
		wantErr      error
	}{
		{
			name:         "happy case",
			statusCode:   http.StatusOK,
			responseBody: []byte{},
			want:         "",
			wantErr:      nil,
		},
		{
			name:         "job not found",
			statusCode:   http.StatusNotFound,
			responseBody: nil,
			want:         "",
			wantErr:      errors.New(""),
		},
		{
			name:         "validation error",
			statusCode:   http.StatusUnprocessableEntity,
			responseBody: nil,
			want:         "",
			wantErr:      errors.New(""),
		},
		{
			name:         "unparseable response",
			statusCode:   http.StatusOK,
			responseBody: []byte{},
			want:         "",
			wantErr:      errors.New(""),
		},
	}
	for _, tt := range testCases {
		// arrange
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.statusCode)
			w.Write(tt.responseBody)
		}))
		defer ts.Close()

		client := New(ts.URL, "user", "key", 3 * time.Second)


		// act
		// _, _ := client.GetBuildForJob(context.Background(), job.Job{})


		// assert
	}
}
