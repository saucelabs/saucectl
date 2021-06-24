package sentry

import (
	setup2 "github.com/saucelabs/saucectl/internal/setup"
	"gotest.tools/v3/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

func Test_attachmentURLFromDSN(t *testing.T) {
	type args struct {
		dsn     string
		eventID string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				dsn:     "https://a5678abc12c590d32fg9f49643072bcf@o448931.ingest.sentry.io/1234567",
				eventID: "123",
			},
			want:    "https://o448931.ingest.sentry.io/api/1234567/events/123/attachments/?sentry_key=a5678abc12c590d32fg9f49643072bcf",
			wantErr: false,
		},
		{
			name: "no schema",
			args: args{
				dsn:     "://a5678abc12c590d32fg9f49643072bcf@o448931.ingest.sentry.io/1234567",
				eventID: "123",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "no user",
			args: args{
				dsn:     "https://o448931.ingest.sentry.io/1234567",
				eventID: "123",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "no project ID",
			args: args{
				dsn:     "https://a5678abc12c590d32fg9f49643072bcf@o448931.ingest.sentry.io",
				eventID: "123",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attachmentURLFromDSN(tt.args.dsn, tt.args.eventID)
			if (err != nil) != tt.wantErr {
				t.Errorf("attachmentURLFromDSN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("attachmentURLFromDSN() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_attach(t *testing.T) {
	file := fs.NewFile(t, "config.yml")
	defer file.Remove()

	var rec *httptest.ResponseRecorder // a new instance is created for each test (see test execution block)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mr, err := req.MultipartReader()
		if err != nil {
			t.Errorf("Failed to create MultipartReader. Request is likely missing multipart data: %v", err)
		}

		if req.URL.Path != "/api/1234567/events/123/attachments/" {
			t.Errorf("Received call on an unexpected path: %s", req.URL.Path)
		}

		p, err := mr.NextPart()
		if err != nil {
			t.Errorf("Failed to retrieve part in multipart: %v", err)
		}

		filename := filepath.Base(file.Path()) // we only care about the name, not the absolute path to it
		if p.FileName() != filename {
			t.Errorf("FileName() got = %s, want = %s", p.FileName(), filename)
		}

		rec.WriteHeader(http.StatusOK)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	purl, err := url.Parse(server.URL)
	if err != nil {
		t.Errorf("Failed to parse server URL: %v", err)
	}
	purl.User = url.User("mockuser")
	purl.Path = "/1234567" // mock project ID
	setup2.SentryDSN = purl.String()

	type args struct {
		client   http.Client
		eventID  string
		filename string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "happy path",
			args: args{
				client:   *server.Client(),
				eventID:  "123",
				filename: file.Path(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec = httptest.NewRecorder() // set a fresh recorder
			attach(*server.Client(), tt.args.eventID, tt.args.filename)
			if rec.Code != http.StatusOK {
				t.Errorf("StatusCode() got = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}
