package rdc

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_ReadAllowedCCY(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		responseBody []byte
		want         int
		wantErr      error
	}{
		{
			name:         "default case",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"organization": { "current": 0, "maximum": 2 }}`),
			want:         2,
			wantErr:      nil,
		},
		{
			name:         "invalid parsing",
			statusCode:   http.StatusOK,
			responseBody: []byte(`{"organization": { "current": 0, "maximum": 2`),
			want:         0,
			wantErr:      errors.New("unexpected EOF"),
		},
		{
			name:         "Forbidden endpoint",
			statusCode:   http.StatusForbidden,
			want:         0,
			wantErr:      errors.New("unexpected statusCode: 403"),
		},
		{
			name:         "error endpoint",
			statusCode:   http.StatusInternalServerError,
			want:         0,
			wantErr:      errors.New("unexpected statusCode: 500"),
		},
	}

	timeout := 3 * time.Second
	for _, tt := range testCases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.statusCode)
			w.Write(tt.responseBody)
		}))

		client := New(ts.URL, "test", "123", timeout)
		ccy, err := client.ReadAllowedCCY(context.Background())
		assert.Equal(t, err, tt.wantErr)
		assert.Equal(t, ccy, tt.want)
		ts.Close()
	}
}
