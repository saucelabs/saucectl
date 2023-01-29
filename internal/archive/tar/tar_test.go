package tar

import (
	archTar "archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/saucelabs/saucectl/internal/sauceignore"
)

func TestArchive(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithDir(".sauce", fs.WithFile("config.yml", "yaml-content", fs.WithMode(0644))),
		fs.WithDir("screenshots", fs.WithFile("screenshot1.png", "foo", fs.WithMode(0755))),
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)))
	defer dir.Remove()

	type wantFile struct {
		isDir        bool
		name         string
		completePath string
	}

	testCases := []struct {
		name      string
		dirName   string
		matcher   sauceignore.Matcher
		options   Options
		wantFiles []wantFile
	}{
		{
			name:    "tar it out",
			dirName: dir.Path(),
			matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			wantFiles: []wantFile{
				{isDir: true, name: "screenshots", completePath: "screenshots"},
				{isDir: false, name: "screenshot1.png", completePath: "screenshots/screenshot1.png"},
				{isDir: false, name: "some.foo.js", completePath: "some.foo.js"},
				{isDir: false, name: "some.other.bar.js", completePath: "some.other.bar.js"},
				{isDir: true, name: ".sauce", completePath: ".sauce"},
				{isDir: false, name: "config.yml", completePath: ".sauce/config.yml"},
			},
		},
		{
			name:    "only one file",
			dirName: filepath.Join(dir.Path(), "some.foo.js"),
			matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			wantFiles: []wantFile{
				{isDir: false, name: "some.foo.js", completePath: "some.foo.js"},
			},
		},
		{
			name:    "tar some.other.bar.js and skip some.foo.js file and screenshots folder",
			dirName: dir.Path(),
			matcher: sauceignore.NewMatcher([]sauceignore.Pattern{
				sauceignore.NewPattern("some.foo.js"),
				sauceignore.NewPattern("screenshots/"),
			}),
			wantFiles: []wantFile{
				{isDir: false, name: "some.other.bar.js", completePath: "some.other.bar.js"},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.dirName)
			if err != nil {
				t.Errorf("unable to open file %s: %v", tt.dirName, err)
			}

			fInfo, err := f.Stat()
			if err != nil {
				t.Errorf("unable to stat file %s: %v", tt.dirName, err)
			}
			
			if fInfo.IsDir() {
				err := os.Chdir(tt.dirName)
				if err != nil {
					t.Errorf("unable to change directory to %s: %v", tt.dirName, err)
				}
			}
			reader, err := Archive(".", tt.matcher, tt.options)
			if err != nil {
				t.Error(err)
			}

			tr := archTar.NewReader(reader)
			foundFiles := make([]bool, len(tt.wantFiles))
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Error(err)
				}

				finfo := header.FileInfo()
				for k, v := range tt.wantFiles {
					if v.name == finfo.Name() {
						assert.Equal(t, v.isDir, finfo.IsDir())
						assert.Equal(t, v.completePath, header.Name)
						foundFiles[k] = true
					}
				}
			}
			// check that all wantFiles are present
			for k, v := range tt.wantFiles {
				if !foundFiles[k] {
					t.Errorf("%s not found in archive", v.name)
				}
			}
		})
	}
}
