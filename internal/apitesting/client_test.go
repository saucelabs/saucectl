package apitesting

import (
	"context"
	"github.com/saucelabs/saucectl/internal/config"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestClient_GetEventResult(t *testing.T) {
	type fields struct {
		HTTPClient *http.Client
		URL        string
		Username   string
		AccessKey  string
	}
	type args struct {
		ctx     context.Context
		hookID  string
		eventID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    TestResult
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				HTTPClient: tt.fields.HTTPClient,
				URL:        tt.fields.URL,
				Username:   tt.fields.Username,
				AccessKey:  tt.fields.AccessKey,
			}
			got, err := c.GetEventResult(tt.args.ctx, tt.args.hookID, tt.args.eventID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEventResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEventResult() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetProject(t *testing.T) {
	type fields struct {
		HTTPClient *http.Client
		URL        string
		Username   string
		AccessKey  string
	}
	type args struct {
		ctx    context.Context
		hookID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    Project
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				HTTPClient: tt.fields.HTTPClient,
				URL:        tt.fields.URL,
				Username:   tt.fields.Username,
				AccessKey:  tt.fields.AccessKey,
			}
			got, err := c.GetProject(tt.args.ctx, tt.args.hookID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_composeURL(t *testing.T) {
	type fields struct {
		HTTPClient *http.Client
		URL        string
		Username   string
		AccessKey  string
	}
	type args struct {
		path    string
		buildID string
		format  string
		tunnel  config.Tunnel
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				HTTPClient: tt.fields.HTTPClient,
				URL:        tt.fields.URL,
				Username:   tt.fields.Username,
				AccessKey:  tt.fields.AccessKey,
			}
			if got := c.composeURL(tt.args.path, tt.args.buildID, tt.args.format, tt.args.tunnel); got != tt.want {
				t.Errorf("composeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	type args struct {
		url       string
		username  string
		accessKey string
		timeout   time.Duration
	}
	tests := []struct {
		name string
		args args
		want Client
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.url, tt.args.username, tt.args.accessKey, tt.args.timeout); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}
