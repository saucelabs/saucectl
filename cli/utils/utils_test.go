package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateOutputPath(t *testing.T) {
	err := ValidateOutputPath("/foo/bar")
	assert.Equal(t, err, errors.New("invalid output path: directory /foo does not exist"))

	err = ValidateOutputPath("")
	assert.Equal(t, err, nil)
}
