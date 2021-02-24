package requesth

import (
	"context"
	"github.com/saucelabs/saucectl/cli/version"
	"io"
	"net/http"
)

func NewWithContext(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return r, err
	}
	r.Header.Set("User-Agent", "saucectl/"+version.Version)

	return r, err
}

func New(method, url string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return r, err
	}
	r.Header.Set("User-Agent", "saucectl/"+version.Version)

	return r, err
}
