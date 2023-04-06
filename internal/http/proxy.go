package http

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"os"
)

// CheckHTTPProxy checks that the HTTP_PROXY is valid if it exists.
func CheckHTTPProxy() error {
	proxyURL := os.Getenv("HTTP_PROXY")
	if proxyURL != "" {
		url, err := url.Parse(proxyURL)
		if err != nil {
			color.Red("\nA proxy has been set, but its url is invalid !\n\n")
			fmt.Printf("%s: %s\n", proxyURL, err)
			println()
			return fmt.Errorf("invalid HTTP_PROXY value")
		}
		log.Info().Msgf(fmt.Sprintf("Using proxy: %s://%s:%s", url.Scheme, url.Hostname(), url.Port()))
	}
	return nil
}

func mustGetProxifiedHTTPTransport() http.RoundTripper {
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
