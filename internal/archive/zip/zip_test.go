package zip

import (
	"archive/zip"
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

	out, err := os.CreateTemp("", "add_test.*.zip")
	if err != nil {
		t.Errorf("failed to create temp file for storing the zip: %v", err)
	}
	defer os.Remove(out.Name())

	sauceignoreOut, err := os.CreateTemp("", "add_test.*.zip")
	if err != nil {
		t.Errorf("failed to create temp file for storing the zip: %v", err)
	}
	defer os.Remove(sauceignoreOut.Name())

	type fields struct {
		W       *zip.Writer
		M       sauceignore.Matcher
		ZipFile *os.File
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
		wantCount int
	}{
		{
			name:      "zip it up",
			fields:    fields{W: zip.NewWriter(out), M: sauceignore.NewMatcher([]sauceignore.Pattern{}), ZipFile: out},
			args:      args{dir.Path(), "", out.Name()},
			wantErr:   false,
			wantFiles: []string{"/screenshot1.png", "/some.foo.js", "/some.other.bar.js"},
			wantCount: 3,
		},
		{
			name: "zip some.other.bar.js and skip some.foo.js file and screenshots folder",
			fields: fields{
				W: zip.NewWriter(sauceignoreOut),
				M: sauceignore.NewMatcher([]sauceignore.Pattern{
					sauceignore.NewPattern("some.foo.js"),
					sauceignore.NewPattern("screenshots/"),
				}),
				ZipFile: sauceignoreOut,
			},
			args:      args{dir.Path(), "", sauceignoreOut.Name()},
			wantErr:   false,
			wantFiles: []string{"/some.other.bar.js"},
			wantCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z := &Writer{
				W:       tt.fields.W,
				M:       tt.fields.M,
				ZipFile: tt.fields.ZipFile,
			}
			fileCount, err := z.Add(tt.args.src, tt.args.dst)
			if (err != nil) != tt.wantErr {
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
			if tt.wantCount != fileCount {
				t.Errorf("got %v, want %v", fileCount, tt.wantCount)
			}

		})
	}
}
