package tar

import (
	archTar "archive/tar"
	"io"
	"strings"
	"testing"

	"gotest.tools/assert"
	"gotest.tools/v3/fs"

	"github.com/saucelabs/saucectl/internal/sauceignore"
)

func TestArchive(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithDir("screenshots", fs.WithFile("screenshot1.png", "foo", fs.WithMode(0755))),
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)))
	defer dir.Remove()

	type wantFile struct {
		isDir bool
		name  string
	}

	testCases := []struct {
		name      string
		matcher   sauceignore.Matcher
		options   Options
		wantFiles []wantFile
	}{
		{
			name:    "tar it out",
			matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			wantFiles: []wantFile{
				{isDir: true, name: "screenshots"},
				{isDir: false, name: "screenshot1.png"},
				{isDir: false, name: "some.foo.js"},
				{isDir: false, name: "some.other.bar.js"},
			},
		},
		{
			name: "tar some.other.bar.js and skip some.foo.js file and screenshots folder",
			matcher: sauceignore.NewMatcher([]sauceignore.Pattern{
				sauceignore.NewPattern("some.foo.js"),
				sauceignore.NewPattern("screenshots/"),
			}),
			wantFiles: []wantFile{
				{isDir: false, name: "some.other.bar.js"},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := Archive(dir.Path(), tt.matcher, tt.options)
			if err != nil {
				t.Error(err)
			}

			tr := archTar.NewReader(reader)
			idx := 0
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Error(err)
				}
				// skip temp dir
				if strings.Contains(header.Name, "tests-") {
					continue
				}

				finfo := header.FileInfo()
				assert.Equal(t, finfo.IsDir(), tt.wantFiles[idx].isDir)
				assert.Equal(t, finfo.Name(), tt.wantFiles[idx].name)
				idx++
			}
		})
	}
}
