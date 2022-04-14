package appstore

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"testing"
	"time"

	"gotest.tools/v3/fs"
)

func TestAppStore_Upload(t *testing.T) {
	dir := fs.NewDir(t, "bundles",
		fs.WithFile("bundle-1.zip", "bundle-1-content", fs.WithMode(0644)),
		fs.WithFile("bundle-2.zip", "bundle-2-content", fs.WithMode(0644)))
	b1 := sha256.New()
	b1.Write([]byte("bundle-1-content"))
	b1Hash := fmt.Sprintf("%x", b1.Sum(nil))
	defer dir.Remove()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fmt.Sprintf("sha256=%s&per_page=1", b1Hash) == r.URL.RawQuery {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"items": [
					{
						"id": "matching-id",
						"sha256": "%s"
					}
				],
				"total_items": 1
			}`, b1Hash)))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"items": [
			],
			"total_items": 0
		}`))
	}))
	defer ts.Close()

	testCases := []struct {
		look    string
		want    string
		wantErr error
	}{
		{look: path.Join(dir.Path(), "bundle-2.zip"), want: "", wantErr: nil},
		{look: path.Join(dir.Path(), "bundle-1.zip"), want: "matching-id", wantErr: nil},
		{look: "", want: "", wantErr: nil},
	}

	as := New(ts.URL, "fake-username", "fake-access-key", 15*time.Second)
	for _, tt := range testCases {
		artifact, err := as.Find(tt.look)

		if !reflect.DeepEqual(err, tt.wantErr) {
			t.Errorf("Error: want: %v, got: %v", tt.wantErr, err)
		}
		if artifact.ID != tt.want {
			t.Errorf("StorageID: want: %v, got: %v", tt.want, artifact.ID)
		}
	}
}
