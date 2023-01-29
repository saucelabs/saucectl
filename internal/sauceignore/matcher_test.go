package sauceignore

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"gotest.tools/v3/assert"
)

func TestPatternsFromFile(t *testing.T) {
	fn, file, err := crtTempSauceignoreFile()
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
			path:            "path/to/not/exists/.sauceignore",
			expectedPatters: []gitignore.Pattern{},
			expectedErr:     nil,
		},
		{
			name: ".sauceignore file is exists",
			path: file,
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
			gotPatters, err := PatternsFromFile(tc.path)
			assert.Equal(t, err, tc.expectedErr)
			assert.Equal(t, len(gotPatters), len(tc.expectedPatters))
		})
	}
}

func TestSauceMatcher(t *testing.T) {
	patterns := []Pattern{
		NewPattern("cypress/videos/"),
		NewPattern("node_modules/"),
		NewPattern(".git/"),
		NewPattern(".gitignore"),
		NewPattern("cypress/test/**"),
		NewPattern("test.txt"),
		NewPattern("*.log"),
		NewPattern("!README.md"),
		NewPattern("videos/**/app.mp4"),
	}

	testCases := []struct {
		name      string
		path      []string
		isDir     bool
		isMatched bool
	}{
		{
			name:      "cypress/videos folder will be ignored",
			path:      []string{"cypress", "videos"},
			isDir:     true,
			isMatched: true,
		}, {
			name:      "node_modules/ folder will be ignored",
			path:      []string{"node_modules"},
			isDir:     true,
			isMatched: true,
		}, {
			name:      ".git/ folder will be ignored",
			path:      []string{".git"},
			isDir:     true,
			isMatched: true,
		}, {
			name:      ".gitignore file will be ignored",
			path:      []string{".gitignore"},
			isDir:     false,
			isMatched: true,
		}, {
			name:      "cypress/test/** folder will be ignored",
			path:      []string{"cypress", "test", "test2"},
			isDir:     true,
			isMatched: true,
		}, {
			name:      "test.txt file will be ignored",
			path:      []string{"test.txt"},
			isDir:     false,
			isMatched: true,
		},
		{
			name:      ".sauceignore file will NOT be ignored",
			path:      []string{".sauceignore"},
			isDir:     false,
			isMatched: false,
		},
		{
			name:      "all log files will be ignored",
			path:      []string{"logs", "cron.log"},
			isDir:     false,
			isMatched: true,
		},
		{
			name:      "README.md files will NOT be ignored",
			path:      []string{"README.md"},
			isDir:     false,
			isMatched: false,
		},
		{
			name:      "subfolder folder will be ignored",
			path:      []string{"videos", "subfolder", "app.mp4"},
			isDir:     false,
			isMatched: true,
		},
	}

	matcher := NewMatcher(patterns)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, matcher.Match(tc.path, tc.isDir), tc.isMatched)
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
	tmpDir, err := os.MkdirTemp("", "sauceignore")
	if err != nil {
		return nil, "", err
	}

	file := filepath.Join(tmpDir, ".sauceignore")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		return nil, "", err
	}

	return func() {
		os.RemoveAll(tmpDir)
	}, file, nil
}

func TestDedupe(t *testing.T) {
	type args struct {
		patterns []Pattern
	}
	tests := []struct {
		name string
		args args
		want []Pattern
	}{
		{
			name: "remove one duplicate",
			args: args{[]Pattern{NewPattern("a"), NewPattern("b"), NewPattern("b")}},
			want: []Pattern{NewPattern("a"), NewPattern("b")},
		},
		{
			name: "no duplicate input",
			args: args{[]Pattern{NewPattern("a"), NewPattern("b"), NewPattern("c")}},
			want: []Pattern{NewPattern("a"), NewPattern("b"), NewPattern("c")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Dedupe(tt.args.patterns); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dedupe() = %v, want %v", got, tt.want)
			}
		})
	}
}
