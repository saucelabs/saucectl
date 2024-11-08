package ioctx

import (
	"context"
	"io"
)

// ContextualReadCloser is a wrapper around an io.ReadCloser that cancels the
// read operation when the context is canceled.
type ContextualReadCloser struct {
	Ctx    context.Context
	Reader io.ReadCloser
}

func (crc ContextualReadCloser) Read(p []byte) (n int, err error) {
	if crc.Ctx.Err() != nil {
		return 0, crc.Ctx.Err()
	}
	return crc.Reader.Read(p)
}

func (crc ContextualReadCloser) Close() error {
	return crc.Reader.Close()
}
