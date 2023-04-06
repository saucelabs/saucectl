package http

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// CheckHTTPProxy checks that the HTTP_PROXY is valid if it exists.
func CheckHTTPProxy() error {
	proxyURL := os.Getenv("HTTP_PROXY")
	if proxyURL != "" {
		if !strings.HasPrefix(proxyURL, "http://") && !strings.HasPrefix(proxyURL, "https://") {
			color.Red("\nA proxy has been set, but its url is invalid !\n\n")
			fmt.Printf("HTTP_PROXY must start by http:// or https:// ")
			return fmt.Errorf("invalid HTTP_PROXY value")
		}
		url, err := url.Parse(proxyURL)
		if err != nil {
			color.Red("\nA proxy has been set, but its url is invalid !\n\n")
			fmt.Printf("%s: %s\n", proxyURL, err)
			println()
			return fmt.Errorf("invalid HTTP_PROXY value")
		}
		displayUrl := proxyURL
		if url.User != nil {
			pass, hasPass := url.User.Password()
			if hasPass {
				displayUrl = strings.Replace(displayUrl, pass, "***", -1)
			}
		}
		log.Info().Msgf(fmt.Sprintf("Using proxy: %s", displayUrl))
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
