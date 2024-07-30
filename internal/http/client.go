package http

import (
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// NewRetryableClient returns a new pre-configured instance of retryablehttp.Client.
func NewRetryableClient(timeout time.Duration) *retryablehttp.Client {
	return &retryablehttp.Client{
		HTTPClient: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 30 * time.Second,
		RetryMax:     3,
		CheckRetry: func(ctx context.Context, resp *http.Response, err error) (bool, error) {
			ok, e := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
			if !ok && resp.StatusCode == http.StatusNotFound {
				return true, nil
			}
			return ok, e
		},
		Backoff:      retryablehttp.DefaultBackoff,
		ErrorHandler: retryablehttp.PassthroughErrorHandler,
	}
}
