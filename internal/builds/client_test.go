package builds

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/stretchr/testify/assert"
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
			responseBody: []byte(`{"id": "happy-build-id"}`),
			want:         "happy-build-id",
			wantErr:      nil,
		},
		{
			name:         "job not found",
			statusCode:   http.StatusNotFound,
			responseBody: nil,
			want:         "",
			wantErr:      errors.New("unexpected statusCode: 404"),
		},
		{
			name:         "validation error",
			statusCode:   http.StatusUnprocessableEntity,
			responseBody: nil,
			want:         "",
			wantErr:      errors.New("unexpected statusCode: 422"),
		},
		{
			name:         "unparseable response",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id": "bad-json-response"`),
			want:         "",
			wantErr:      errors.New("unexpected EOF"),
		},
	}
	for _, tt := range testCases {
		// arrange
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.statusCode)
			w.Write(tt.responseBody)
		}))
		defer ts.Close()

		client := New(ts.URL, "user", "key", 3*time.Second)

		// act
		bid, err := client.GetBuildIdForJob(context.Background(), job.Job{})

		// assert
		assert.Equal(t, bid, tt.want)
		if err != nil {
			assert.True(t, strings.Contains(err.Error(), tt.wantErr.Error()))
		}
	}
}
