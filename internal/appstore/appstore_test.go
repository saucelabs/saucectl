package appstore

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"regexp"
	"testing"
	"time"

	"gotest.tools/v3/fs"
)

func TestAppStore_Upload(t *testing.T) {
	dir := fs.NewDir(t, "bundles",
		fs.WithFile("bundle-1.zip", "bundle-1-content", fs.WithMode(0644)),
		fs.WithFile("bundle-2.zip", "bundle-2-content", fs.WithMode(0644)),
		fs.WithFile("bundle-3.zip", "bundle-3-content", fs.WithMode(0644)))
	b1 := md5.New()
	b1.Write([]byte("bundle-1-content"))
	b1Hash := fmt.Sprintf("%x", b1.Sum(nil))
	b2 := md5.New()
	b2.Write([]byte("bundle-2-content"))
	b2Hash := fmt.Sprintf("%x", b2.Sum(nil))
	defer dir.Remove()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		completeQuery := fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
		re := regexp.MustCompile(`name=([^&]+)`)
		filename := re.FindStringSubmatch(r.URL.RawQuery)[1]
		if matched, _ := regexp.Match(`.*page=2.*`, []byte(r.URL.RawQuery)); matched {
			w.WriteHeader(200)
			w.Write([]byte(fmt.Sprintf(`{"items": [{"id":"matching-id-next", "etag": "%s"}]}`, b2Hash)))
			return
		}
		if matched, _ := regexp.Match(`\/v1\/storage\/list\?name=.*&per_page=100`, []byte(completeQuery)); matched {
			w.WriteHeader(200)
			w.Write([]byte(fmt.Sprintf(`{"items": [{"id":"matching-id", "etag": "%s"}], "links": {"next": "?name=%s&per_page=100&page=2"}}`, b1Hash, filename)))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	testCases := []struct {
		look    string
		want    string
		wantErr error
	}{
		{look: path.Join(dir.Path(), "bundle-3.zip"), want: "", wantErr: nil},
		{look: path.Join(dir.Path(), "bundle-1.zip"), want: "matching-id", wantErr: nil},
		{look: path.Join(dir.Path(), "bundle-2.zip"), want: "matching-id-next", wantErr: nil},
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
