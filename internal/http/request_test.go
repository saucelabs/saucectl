package http

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestNewRequestWithContext(t *testing.T) {
	type args struct {
		ctx    context.Context
		method string
		url    string
		body   io.Reader
	}
	tests := []struct {
		name        string
		args        args
		wantHeaders http.Header
		wantErr     bool
	}{
		{
			name: "expect headers",
			args: args{
				ctx:    context.Background(),
				method: "GET",
				url:    "http://localhost",
				body:   nil,
			},
			wantHeaders: http.Header{"User-Agent": []string{"saucectl/0.0.0+unknown"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRequestWithContext(tt.args.ctx, tt.args.method, tt.args.url, tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRequestWithContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.Header, tt.wantHeaders) {
				t.Errorf("Headers got = %v, want %v", got, tt.wantHeaders)
			}
		})
	}
}
