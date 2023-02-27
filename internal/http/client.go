package http

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// NewRetryableClient returns a new pre-configured instance of retryablehttp.Client.
func NewRetryableClient(timeout time.Duration) *retryablehttp.Client {
	return &retryablehttp.Client{
		HTTPClient:   &http.Client{Timeout: timeout},
		RetryWaitMin: 1 * time.Second,
		RetryWaitMax: 30 * time.Second,
		RetryMax:     3,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
		ErrorHandler: retryablehttp.PassthroughErrorHandler,
	}
}
