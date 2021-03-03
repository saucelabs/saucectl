package requesth

import (
	"context"
	"github.com/saucelabs/saucectl/cli/version"
	"io"
	"net/http"
)

// NewWithContext is a wrapper around http.NewRequestWithContext that modifies the request by adding additional
// headers.
func NewWithContext(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return r, err
	}
	r.Header.Set("User-Agent", "saucectl/"+version.Version)

	return r, err
}

// New is a wrapper around NewWithConext.
func New(method, url string, body io.Reader) (*http.Request, error) {
	return NewWithContext(context.Background(), method, url, body)
}
