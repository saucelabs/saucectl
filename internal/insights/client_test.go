package insights

import (
	"context"
	"github.com/saucelabs/saucectl/internal/credentials"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_PostTestRun(t *testing.T) {
	tests := []struct {
		name    string
		runs    []TestRun
		reply   func(t *testing.T) func(w http.ResponseWriter, r *http.Request)
		wantErr bool
	}{
		{
			name: "Basic - empty - 204",
			runs: []TestRun{},
			reply: func(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(204)
				}
			},
			wantErr: false,
		},
		{
			name: "Basic - Erroring - 422",
			runs: []TestRun{
				{
					ID: "09a87dea-3923-43db-8743-ef1f3ff5d717",
				},
			},
			reply: func(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(204)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		ts := httptest.NewServer(http.HandlerFunc(tt.reply(t)))

		t.Run(tt.name, func(t *testing.T) {
			c := &InsightsService{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: credentials.Credentials{AccessKey: "accessKey", Username: "username"},
			}
			if err := c.PostTestRun(context.Background(), tt.runs); (err != nil) != tt.wantErr {
				t.Errorf("PostTestRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})

		ts.Close()
	}
}
