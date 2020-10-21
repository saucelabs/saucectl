package utils

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateOutputPath(t *testing.T) {
	err := ValidateOutputPath("/foo/bar")
	assert.Equal(t, err, errors.New("invalid output path: directory /foo does not exist"))

	err = ValidateOutputPath("")
	assert.Equal(t, err, nil)
}

func TestValidateOutputPathFileMode(t *testing.T) {
	assert.Equal(t, ValidateOutputPathFileMode(os.FileMode(1<<(32-1-4))), nil)
	assert.Equal(t, ValidateOutputPathFileMode(os.FileMode(1<<(32-1-5))), errors.New("got a device"))
	assert.Equal(t, ValidateOutputPathFileMode(os.FileMode(1<<(32-1-12))), errors.New("got an irregular file"))
}

func TestGetHomeDir(t *testing.T) {
	cwd, _ := os.Getwd()
	cases := []struct {
		SauceRootDir	string
		SauceVM			string
		expected		string
		shouldPass		bool
	} {
		{"/path/to/root/dir", "", "/path/to/root/dir", true},
		{"", "", "/home/seluser", true},
		{"", "truthy", cwd, true},
	}
	for _, tc := range cases {
		os.Setenv("SAUCE_ROOT_DIR", tc.SauceRootDir)
		os.Setenv("SAUCE_VM", tc.SauceVM)
		homeDir := GetProjectDir()
		assert.Equal(t, homeDir, tc.expected)
	}
}
