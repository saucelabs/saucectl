package appstore

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/xtgo/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestAppStore_UploadStream(t *testing.T) {
	// mock test values
	itemID := uuid.NewRandom().String()
	itemName := "hello.txt"
	uploadTimestamp := time.Now().Round(1 * time.Second)
	testUser := "test"
	testPass := "test"

	dir := fs.NewDir(t, "checksums", fs.WithFile(itemName, "world!"))
	defer dir.Remove()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/storage/upload" {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("incorrect path"))
			return
		}

		user, pass, _ := r.BasicAuth()
		if user != testUser || pass != testPass {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(http.StatusText(http.StatusForbidden)))
		}

		if err := r.ParseForm(); err != nil {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "failed to parse form post: %v", err)
		}

		reader, err := r.MultipartReader()
		if err != nil {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "failed to read multipart form: %v", err)
		}

		p, err := reader.NextPart()
		if err == io.EOF {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "unexpected early end of multipart data: %v", err)
		}
		if err != nil {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "failed to retrieve next part in multipart: %v", err)
		}

		_, _ = io.Copy(io.Discard, p)

		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(UploadResponse{Item{
			ID:              itemID,
			Name:            p.FileName(),
			UploadTimestamp: uploadTimestamp.Unix(),
		}})
	}))
	defer server.Close()

	type fields struct {
		HTTPClient *http.Client
		URL        string
		Username   string
		AccessKey  string
	}
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    storage.Item
		wantErr bool
	}{
		{
			name: "successfully upload file",
			fields: fields{
				HTTPClient: &http.Client{Timeout: 10 * time.Second},
				URL:        server.URL,
				Username:   testUser,
				AccessKey:  testPass,
			},
			args: args{dir.Join("hello.txt")},
			want: storage.Item{
				ID:       itemID,
				Name:     "hello.txt",
				Uploaded: uploadTimestamp,
			},
			wantErr: false,
		},
		{
			name: "wrong credentials",
			fields: fields{
				HTTPClient: &http.Client{Timeout: 10 * time.Second},
				URL:        server.URL,
				Username:   testUser + "1",
				AccessKey:  testPass + "1",
			},
			args:    args{dir.Join("hello.txt")},
			want:    storage.Item{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AppStore{
				HTTPClient: tt.fields.HTTPClient,
				URL:        tt.fields.URL,
				Username:   tt.fields.Username,
				AccessKey:  tt.fields.AccessKey,
			}

			f, err := os.Open(tt.args.filename)
			if err != nil {
				t.Error(err)
			}
			defer func(f *os.File) {
				_ = f.Close()
			}(f)

			got, err := s.UploadStream(tt.args.filename, f)
			if (err != nil) != tt.wantErr {
				t.Errorf("UploadStream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UploadStream() got = %v, want %v", got, tt.want)
			}
		})
	}
}
