package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpinnerSingleton(t *testing.T) {
	spinner := NewSpinner()
	spinner2 := NewSpinner()
	assert.Equal(t, spinner, spinner2)
}
