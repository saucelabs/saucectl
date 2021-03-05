package zip

import (
	"archive/zip"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/fs"

	"github.com/saucelabs/saucectl/internal/sauceignore"
)

func TestZipper_Add(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithDir("screenshots", fs.WithFile("screenshot1.png", "foo", fs.WithMode(0755))),
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)))
	defer dir.Remove()

	out, err := ioutil.TempFile(os.TempDir(), "add_test.*.zip")
	if err != nil {
		t.Errorf("failed to create temp file for storing the zip: %v", err)
	}
	defer os.Remove(out.Name())

	sauceignoreOut, err := ioutil.TempFile(os.TempDir(), "add_test.*.zip")
	if err != nil {
		t.Errorf("failed to create temp file for storing the zip: %v", err)
	}
	defer os.Remove(sauceignoreOut.Name())

	type fields struct {
		W *zip.Writer
		M sauceignore.Matcher
	}
	type args struct {
		src     string
		dst     string
		outName string
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
			fields:    fields{W: zip.NewWriter(out), M: sauceignore.NewMatcher([]sauceignore.Pattern{})},
			args:      args{dir.Path(), "", out.Name()},
			wantErr:   false,
			wantFiles: []string{"/screenshot1.png", "/some.foo.js", "/some.other.bar.js"},
		},
		{
			name: "zip some.other.bar.js and skip some.foo.js file and screenshots folder",
			fields: fields{
				W: zip.NewWriter(sauceignoreOut),
				M: sauceignore.NewMatcher([]sauceignore.Pattern{
					sauceignore.NewPattern("some.foo.js"),
					sauceignore.NewPattern("screenshots/"),
				})},
			args:      args{dir.Path(), "", sauceignoreOut.Name()},
			wantErr:   false,
			wantFiles: []string{"/some.other.bar.js"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z := &Writer{
				W: tt.fields.W,
				M: tt.fields.M,
			}
			if err := z.Add(tt.args.src, tt.args.dst); (err != nil) != tt.wantErr {
				t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := z.Close(); err != nil {
				t.Errorf("failed to close archive: %v", err)
			}

			r, err := zip.OpenReader(tt.args.outName)
			if err != nil {
				t.Errorf("failed to open %v: %v", tt.args.outName, err)
			}

			for i, f := range r.File {
				if !strings.Contains(f.Name, tt.wantFiles[i]) {
					t.Errorf("got %v, want %v", f.Name, tt.wantFiles[i])
				}
			}
		})
	}
}
