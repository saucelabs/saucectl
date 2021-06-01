package appstore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestAppStore_Upload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		completeQuery := fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
		switch completeQuery {
		case "/v1/storage/list?":
			w.WriteHeader(200)
			w.Write([]byte(`{"items": [{"id":"matching-id", "etag": "matching-hash"}], "links": {"next": "?page=2"}}`))
		case "/v1/storage/list?page=2":
			w.WriteHeader(200)
			w.Write([]byte(`{"items": [{"id":"matching-id-next", "etag": "matching-hash-next"}]}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	testCases := []struct {
		look    string
		want    string
		wantErr error
	}{
		{look: "not-found-hash", want: "", wantErr: nil},
		{look: "matching-hash", want: "matching-id", wantErr: nil},
		{look: "matching-hash-next", want: "matching-id-next", wantErr: nil},
		{look: "", want: "", wantErr: nil},
	}

	as := New(ts.URL, "fake-username", "fake-access-key", 15*time.Second)
	for _, tt := range testCases {
		artifact, err := as.Locate(tt.look)

		if !reflect.DeepEqual(err, tt.wantErr) {
			t.Errorf("Error: want: %v, got: %v", tt.wantErr, err)
		}
		if artifact.ID != tt.want {
			t.Errorf("StorageID: want: %v, got: %v", tt.want, artifact.ID)
		}
	}
}
