package main

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSetupLogging(t *testing.T) {
	setupLogging(true)
	assert.Equal(t, zerolog.GlobalLevel(), zerolog.Level(1))
	setupLogging(false)
	assert.Equal(t, zerolog.GlobalLevel(), zerolog.Level(3))
}
