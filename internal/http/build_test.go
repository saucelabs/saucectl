package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/build"
	"github.com/stretchr/testify/assert"
)

func TestBuildService_GetBuildID(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		responseBody []byte
		want         build.Build
		wantErr      error
	}{
		{
			name:         "happy case",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id": "happy-build-id"}`),
			want:         build.Build{ID: "happy-build-id"},
			wantErr:      nil,
		},
		{
			name:         "job not found",
			statusCode:   http.StatusNotFound,
			responseBody: nil,
			want:         build.Build{},
			wantErr:      errors.New("unexpected statusCode: 404"),
		},
		{
			name:         "validation error",
			statusCode:   http.StatusUnprocessableEntity,
			responseBody: nil,
			want:         build.Build{},
			wantErr:      errors.New("unexpected statusCode: 422"),
		},
		{
			name:         "unparseable response",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"id": "bad-json-response"`),
			want:         build.Build{},
			wantErr:      errors.New("unexpected EOF"),
		},
	}
	for _, tt := range testCases {
		// arrange
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tt.statusCode)
			_, _ = w.Write(tt.responseBody)
		}))
		defer ts.Close()

		client := NewBuildService(ts.URL, "user", "key", 3*time.Second)
		client.Client.RetryWaitMax = 1 * time.Millisecond

		// act
		bid, err := client.FindBuild(
			context.Background(), "some-job-id", false,
		)

		// assert
		assert.Equal(t, bid, tt.want)
		if err != nil {
			assert.True(t, strings.Contains(err.Error(), tt.wantErr.Error()))
		}
	}
}
