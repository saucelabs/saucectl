package http

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/msg"
)

var (
	// ErrServerError is returned when the server was not able to correctly handle our request (status code >= 500).
	ErrServerError = errors.New(msg.InternalServerError)
	// ErrJobNotFound is returned when the requested job was not found.
	ErrJobNotFound = errors.New(msg.JobNotFound)
	// ErrAssetNotFound is returned when the requested asset was not found.
	ErrAssetNotFound = errors.New(msg.AssetNotFound)
	// ErrTunnelNotFound is returned when the requested tunnel was not found.
	ErrTunnelNotFound = errors.New(msg.TunnelNotFound)
)
