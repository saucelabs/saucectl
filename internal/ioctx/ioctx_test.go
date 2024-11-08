package ioctx

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestContextualReadCloser_Read(t *testing.T) {
	type fields struct {
		Ctx    context.Context
		Reader io.ReadCloser
	}
	tests := []struct {
		name      string
		fields    fields
		wantN     int64
		wantErr   bool
		cancelCtx bool
	}{
		{
			name: "read_all",
			fields: fields{
				Ctx:    context.Background(),
				Reader: io.NopCloser(strings.NewReader("hello")),
			},
			wantN:     5,
			wantErr:   false,
			cancelCtx: false,
		},
		{
			name: "cancel_read",
			fields: fields{
				Ctx:    context.Background(),
				Reader: io.NopCloser(strings.NewReader("hello")),
			},
			wantN:     0,
			wantErr:   true,
			cancelCtx: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crc := ContextualReadCloser{
				Ctx:    tt.fields.Ctx,
				Reader: tt.fields.Reader,
			}

			if tt.cancelCtx {
				ctx, cancel := context.WithCancel(crc.Ctx)
				crc.Ctx = ctx
				cancel()
			}

			gotN, err := io.Copy(io.Discard, crc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotN != tt.wantN {
				t.Errorf("Read() gotN = %v, want %v", gotN, tt.wantN)
			}
		})
	}
}
