package http

import (
	"net/http"
	"net/url"
	"os"
)

func getProxifiedHttpTransport() http.RoundTripper {
	proxyURL := os.Getenv("HTTP_PROXY")
	if proxyURL == "" {
		return nil
	}
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		// Note: panic() as it is checked right after startup.
		panic("HTTP_PROXY parsing failure")
	}
	return &http.Transport{
		Proxy: http.ProxyURL(parsedURL),
	}
}
