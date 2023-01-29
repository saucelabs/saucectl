package main

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSetupLogging(t *testing.T) {
	setupLogging(true, true)
	assert.Equal(t, zerolog.GlobalLevel(), zerolog.DebugLevel)
	setupLogging(false, true)
	assert.Equal(t, zerolog.GlobalLevel(), zerolog.InfoLevel)
}
