package mocks

import "context"

// CCYReader is a mock implemention for the concurrency.Reader interface.
type CCYReader struct {
	ReadAllowedCCYfn func(ctx context.Context) (int, error)
}

// ReadAllowedCCY is a wrapper around CCYReader.ReadAllowedCCYfn.
func (r CCYReader) ReadAllowedCCY(ctx context.Context) (int, error) {
	return r.ReadAllowedCCYfn(ctx)
}
