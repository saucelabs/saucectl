package zip

import (
	"archive/zip"
	"fmt"
	"gotest.tools/v3/fs"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestZipper_Add(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)))
	defer dir.Remove()

	out, err := ioutil.TempFile(os.TempDir(), "add_test.*.zip")
	if err != nil {
		t.Errorf("failed to create temp file for storing the zip: %v", err)
	}
	defer os.Remove(out.Name())

	type fields struct {
		W *zip.Writer
	}
	type args struct {
		src string
		dst string
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantErr   bool
		wantFiles []string
	}{
		{
			name:      "zip it up",
			fields:    fields{W: zip.NewWriter(out)},
			args:      args{dir.Path(), ""},
			wantErr:   false,
			wantFiles: []string{"/some.foo.js", "/some.other.bar.js"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z := &Writer{
				W: tt.fields.W,
			}
			if err := z.Add(tt.args.src, tt.args.dst); (err != nil) != tt.wantErr {
				t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := z.Close(); err != nil {
				t.Errorf("failed to close archive: %v", err)
			}

			r, err := zip.OpenReader(out.Name())
			if err != nil {
				t.Errorf("failed to open %v: %v", out.Name(), err)
			}

			for i, f := range r.File {
				if !strings.Contains(f.Name, tt.wantFiles[i]) {
					fmt.Printf("got %v, want %v", f.Name, tt.wantFiles[i])
				}
			}
		})
	}
}
