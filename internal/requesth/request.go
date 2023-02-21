package requesth

import (
	"context"
	"io"
	"net/http"

	"github.com/saucelabs/saucectl/internal/version"
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
