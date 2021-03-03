package sauceignore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"gotest.tools/assert"
)

func TestReadIgnoreFile(t *testing.T) {
	fn, sauceIgnorePath, err := crtTempSauceignoreFile()
	if err != nil {
		t.Fatalf("couldn't create temp .sauceignore file %s", err)
	}
	// remove temp folder with temp .sauceignore file
	if fn != nil {
		defer fn()
	}

	testsCases := []struct {
		name            string
		path            string
		expectedPatters []gitignore.Pattern
		expectedErr     error
	}{
		{
			name:            ".sauceignore file is not exists",
			path:            "path/to/not/exists/folder",
			expectedPatters: []gitignore.Pattern{},
			expectedErr:     nil,
		},
		{
			name: ".sauceignore file is exists",
			path: sauceIgnorePath,
			expectedPatters: []gitignore.Pattern{
				gitignore.ParsePattern("cypress/screenshots/", nil),
				gitignore.ParsePattern("cypress/videos/", nil),
				gitignore.ParsePattern("node_modules/", nil),
				gitignore.ParsePattern(".git/", nil),
				gitignore.ParsePattern(".github/", nil),
				gitignore.ParsePattern(".DS_Store", nil),
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			gotPatters, err := ReadIgnoreFile(tc.path)
			assert.Equal(t, err, tc.expectedErr)
			assert.Equal(t, len(gotPatters), len(tc.expectedPatters))
		})
	}
}

func crtTempSauceignoreFile() (func(), string, error) {
	content := `cypress/screenshots/
cypress/videos/
# Remove this to have node_modules uploaded with code
node_modules/
.git/
# some_folder/
.github/
.DS_Store`
	tmpDir, err := ioutil.TempDir("", "sauceignore")
	if err != nil {
		return nil, "", err
	}

	if err := ioutil.WriteFile(filepath.Join(tmpDir, sauceignore), []byte(content), 0644); err != nil {
		return nil, "", err
	}

	return func() {
		os.RemoveAll(tmpDir)
	}, tmpDir, nil
}
