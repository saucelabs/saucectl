package utils

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSplitLines(t *testing.T) {
	lines := SplitLines("/foo/bar\n/bar/foo")
	assert.Equal(t, lines, []string{"/foo/bar", "/bar/foo"})
}
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
