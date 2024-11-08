package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/xtgo/uuid"

	"gotest.tools/v3/fs"
)

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

		size, _ := io.Copy(io.Discard, p)

		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(UploadResponse{Item{
			ID:              itemID,
			Name:            p.FileName(),
			UploadTimestamp: uploadTimestamp.Unix(),
			Size:            int(size),
		}})
	}))
	defer server.Close()

	type fields struct {
		HTTPClient *retryablehttp.Client
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
				HTTPClient: NewRetryableClient(10 * time.Second),
				URL:        server.URL,
				Username:   testUser,
				AccessKey:  testPass,
			},
			args: args{dir.Join("hello.txt")},
			want: storage.Item{
				ID:       itemID,
				Name:     "hello.txt",
				Uploaded: uploadTimestamp,
				Size:     6,
			},
			wantErr: false,
		},
		{
			name: "wrong credentials",
			fields: fields{
				HTTPClient: NewRetryableClient(10 * time.Second),
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

			got, err := s.UploadStream(context.Background(), storage.FileInfo{Name: tt.args.filename}, f)
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
		HTTPClient *retryablehttp.Client
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
				HTTPClient: NewRetryableClient(10 * time.Second),
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
				HTTPClient: NewRetryableClient(10 * time.Second),
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
				HTTPClient: NewRetryableClient(10 * time.Second),
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
			got, err := s.List(context.Background(), tt.args.opts)
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

func TestAppStore_Delete(t *testing.T) {
	testUser := "test"
	testPass := "test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/v1/storage/files/") {
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte("incorrect path"))
			return
		}
		println(path.Base(r.URL.Path))
		if path.Base(r.URL.Path) == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("missing file id"))
			return
		}

		user, pass, _ := r.BasicAuth()
		if user != testUser || pass != testPass {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(http.StatusText(http.StatusForbidden)))
			return
		}

		w.WriteHeader(200)
		// The real server's response body contains a JSON that describes the
		// deleted item. We don't need that for this test.
	}))
	defer server.Close()

	type fields struct {
		HTTPClient *retryablehttp.Client
		URL        string
		Username   string
		AccessKey  string
	}
	type args struct {
		id string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "delete item successfully",
			fields: fields{
				HTTPClient: NewRetryableClient(10 * time.Second),
				URL:        server.URL,
				Username:   testUser,
				AccessKey:  testPass,
			},
			args:    args{id: uuid.NewRandom().String()},
			wantErr: assert.NoError,
		},
		{
			name: "fail on wrong credentials",
			fields: fields{
				HTTPClient: NewRetryableClient(10 * time.Second),
				URL:        server.URL,
				Username:   testUser + "1",
				AccessKey:  testPass + "1",
			},
			args:    args{id: uuid.NewRandom().String()},
			wantErr: assert.Error,
		},
		{
			name: "fail when no ID was specified",
			fields: fields{
				HTTPClient: NewRetryableClient(10 * time.Second),
				URL:        server.URL,
				Username:   testUser,
				AccessKey:  testPass,
			},
			args:    args{id: ""},
			wantErr: assert.Error,
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
			tt.wantErr(t, s.Delete(context.Background(), tt.args.id), fmt.Sprintf("Delete(%v)", tt.args.id))
		})
	}
}
