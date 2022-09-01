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
			return
		}

		if err := r.ParseForm(); err != nil {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "failed to parse form post: %v", err)
			return
		}

		reader, err := r.MultipartReader()
		if err != nil {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "failed to read multipart form: %v", err)
			return
		}

		p, err := reader.NextPart()
		if err == io.EOF {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "unexpected early end of multipart data: %v", err)
			return
		}
		if err != nil {
			w.WriteHeader(400)
			_, _ = fmt.Fprintf(w, "failed to retrieve next part in multipart: %v", err)
			return
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

func TestAppStore_List(t *testing.T) {
	testUser := "test"
	testPass := "test"

	// Items that are known to the mock server.
	items := []Item{
		{
			ID:              uuid.NewRandom().String(),
			Name:            "hello.app",
			UploadTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		},
		{
			ID:              uuid.NewRandom().String(),
			Name:            "world.app",
			UploadTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/storage/files" {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("incorrect path"))
			return
		}

		user, pass, _ := r.BasicAuth()
		if user != testUser || pass != testPass {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(http.StatusText(http.StatusForbidden)))
			return
		}

		var filteredItems []Item
		filtered := false

		nameQuery := r.URL.Query().Get("name")
		if r.URL.Query().Get("name") != "" {
			filtered = true
			for _, v := range items {
				if v.Name == nameQuery {
					filteredItems = append(filteredItems, v)
				}
			}
		}

		// Return all items if no filter was applied.
		if !filtered {
			filteredItems = items
		}

		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(ListResponse{Items: filteredItems})
	}))
	defer server.Close()

	type fields struct {
		HTTPClient *http.Client
		URL        string
		Username   string
		AccessKey  string
	}
	type args struct {
		opts storage.ListOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    storage.List
		wantErr bool
	}{
		{
			name: "query all",
			fields: fields{
				HTTPClient: &http.Client{Timeout: 10 * time.Second},
				URL:        server.URL,
				Username:   testUser,
				AccessKey:  testPass,
			},
			args: args{},
			want: storage.List{
				Items: []storage.Item{
					{
						ID:       items[0].ID,
						Name:     items[0].Name,
						Size:     items[0].Size,
						Uploaded: time.Unix(items[0].UploadTimestamp, 0),
					},
					{
						ID:       items[1].ID,
						Name:     items[1].Name,
						Size:     items[1].Size,
						Uploaded: time.Unix(items[1].UploadTimestamp, 0),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "query subset",
			fields: fields{
				HTTPClient: &http.Client{Timeout: 10 * time.Second},
				URL:        server.URL,
				Username:   testUser,
				AccessKey:  testPass,
			},
			args: args{
				opts: storage.ListOptions{Name: items[0].Name},
			},
			want: storage.List{
				Items: []storage.Item{
					{
						ID:       items[0].ID,
						Name:     items[0].Name,
						Size:     items[0].Size,
						Uploaded: time.Unix(items[0].UploadTimestamp, 0),
					},
				},
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
			args:    args{},
			want:    storage.List{},
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
			got, err := s.List(tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() got = %v, want %v", got, tt.want)
			}
		})
	}
}
