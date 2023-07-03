package sauceignore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func Test_ExcludeSauceIgnorePatterns(t *testing.T) {
	tests := []struct {
		name               string
		files              []string
		sauceIgnoreContent string
		want               []string
	}{
		{
			name: "No Filter",
			files: []string{
				"tests/dir1/file1.js",
				"tests/dir2/file1.js",
				"tests/dir2/file2.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
				"tests/example/node_modules/bin/test.js",
			},
			sauceIgnoreContent: "",
			want: []string{
				"tests/dir1/file1.js",
				"tests/dir2/file1.js",
				"tests/dir2/file2.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
				"tests/example/node_modules/bin/test.js",
			},
		},
		{
			name: "node_modules excluded",
			files: []string{
				"tests/dir1/file1.js",
				"tests/dir2/file1.js",
				"tests/dir2/file2.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
				"tests/example/node_modules/bin/test.js",
			},
			sauceIgnoreContent: "node_modules\n",
			want: []string{
				"tests/dir1/file1.js",
				"tests/dir2/file1.js",
				"tests/dir2/file2.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
			},
		},
		{
			name: "dir2 level excluded",
			files: []string{
				"tests/dir1/file1.js",
				"tests/dir2/file1.js",
				"tests/dir2/file2.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
				"tests/example/node_modules/bin/test.js",
			},
			sauceIgnoreContent: "dir2",
			want: []string{
				"tests/dir1/file1.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
				"tests/example/node_modules/bin/test.js",
			},
		},
		{
			name: "file1 excluded",
			files: []string{
				"tests/dir1/file1.js",
				"tests/dir2/file1.js",
				"tests/dir2/file2.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
				"tests/example/node_modules/bin/test.js",
			},
			sauceIgnoreContent: "file1.js",
			want: []string{
				"tests/dir2/file2.js",
				"tests/dir3/file3.js",
				"tests/dir4/file4.js",
				"tests/example/node_modules/bin/test.js",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fs.NewDir(t, "testcafe", fs.WithFile(".sauceignore", tt.sauceIgnoreContent, fs.WithMode(0644)))
			defer dir.Remove()
			assert.Equalf(t, tt.want, ExcludeSauceIgnorePatterns(tt.files, dir.Join(".sauceignore")), "excludeSauceIgnorePatterns(%v, {%v})", tt.files, tt.sauceIgnoreContent)
		})
	}
}
