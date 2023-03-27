package http

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestImageRunner_GetArtifacts(t *testing.T) {
	type fields struct {
		Client *retryablehttp.Client
		URL    string
		Creds  iam.Credentials
	}
	type args struct {
		ctx context.Context
		id  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    assert.ValueAssertionFunc
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:   "Empty Payload",
			fields: fields{},
			args: args{
				ctx: context.Background(),
				id:  "run-id-1",
			},
			want: func(t assert.TestingT, i interface{}, input ...interface{}) bool {
				id := i.(string)
				fd, err := os.Open(id)
				defer fd.Close()
				if err != nil {
					return false
				}
				body, _ := io.ReadAll(fd)
				return string(body) == "expected-run-1"
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err == nil {
					return true
				}
				return false
			},
		},
	}
	ta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/artifacts/run-id-1":
			w.WriteHeader(200)
			_, err = w.Write([]byte("expected-run-1"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ta.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/v1alpha1/hosted/image/runners/run-id-1/artifacts/url":
			w.WriteHeader(200)
			_, err = w.Write([]byte(fmt.Sprintf(`{"url":"%s/artifacts/run-id-1"}`, ta.URL)))
			break
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ImageRunner{
				Client: retryablehttp.NewClient(),
				URL:    ts.URL,
				Creds:  tt.fields.Creds,
			}
			got, err := c.DownloadArtifacts(tt.args.ctx, tt.args.id)
			if err != nil {
			}
			if !tt.wantErr(t, err, fmt.Sprintf("GetArtifacts(%v, %v)", tt.args.ctx, tt.args.id)) {
				return
			}
			tt.want(t, got, fmt.Sprintf("GetArtifacts(%v, %v)", tt.args.ctx, tt.args.id))
		})
	}
}
