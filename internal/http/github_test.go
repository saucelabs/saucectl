package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGitHub_isUpdateRequired(t *testing.T) {
	gh := GitHub{}

	testCases := []struct {
		current string
		remote  string
		want    bool
	}{
		{current: "v0.1.0", remote: "v0.1.1", want: true},
		{current: "v0.2.0", remote: "v0.1.1", want: false},
		{current: "v0.1.0", remote: "v0.1.0", want: false},
		{current: "0.1.0", remote: "v0.1.1", want: true},
		{current: "0.2.0", remote: "v0.1.1", want: false},
		{current: "0.1.0", remote: "v0.1.0", want: false},
		{current: "v0.1.0", remote: "0.1.1", want: true},
		{current: "v0.2.0", remote: "0.1.1", want: false},
		{current: "v0.1.0", remote: "0.1.0", want: false},
		{current: "0.1.0", remote: "0.1.1", want: true},
		{current: "0.2.0", remote: "0.1.1", want: false},
		{current: "0.1.0", remote: "0.1.0", want: false},
		{current: "v0.0.0+unknown", remote: "v0.1.0", want: true},
	}
	for _, tt := range testCases {
		got := gh.isUpdateRequired(tt.current, tt.remote)
		if tt.want != got {
			t.Errorf("%s <=> %s, want: %v, got: %v", tt.current, tt.remote, tt.want, got)
		}
	}
}

func TestGitHub_IsUpdateAvailable(t *testing.T) {
	testCases := []struct {
		body    []byte
		current string
		want    string
		wantErr error
	}{
		{
			body:    []byte(`{"tag_name": "v0.43.0", "name": "v0.43.0"}`),
			current: "v0.1.0",
			want:    "v0.43.0",
			wantErr: nil,
		},
		{
			body:    []byte(`{"tag_name": "v0.43.0", "name": "v0.43.0"}`),
			current: "v0.44.0",
			want:    "",
			wantErr: nil,
		},
		{
			body:    []byte(`{"tag_name": "v0.43.0", "name": "v0.43.0"}`),
			current: "v0.0.0+unknown",
			want:    "v0.43.0",
			wantErr: nil,
		},
	}
	for idx, tt := range testCases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			switch r.URL.Path {
			case "/repos/saucelabs/saucectl/releases/latest":
				w.WriteHeader(200)
				_, err = w.Write(tt.body)
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}

			if err != nil {
				t.Errorf("%d: failed to respond: %v", idx, err)
			}
		}))
		gh := GitHub{
			HTTPClient: &http.Client{Timeout: 1 * time.Second},
			URL:        ts.URL,
		}

		// Forcing current version
		got, err := gh.IsUpdateAvailable(tt.current)

		if err != tt.wantErr {
			t.Errorf("Case %d (err): want: %v, got: %v", idx, tt.wantErr, err)
		}
		if got != tt.want {
			t.Errorf("Case %d: want: %v, got: %v", idx, tt.want, got)
		}
		ts.Close()
	}
}
