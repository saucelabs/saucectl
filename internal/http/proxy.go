package http

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
)

// Note: The verification logic is borrowed from golang.org/x/net/http/httpproxy.
// Those useful functions are not exposed, but are required to allow us to warn
// the user upfront.
func getEnvAny(names ...string) string {
	for _, n := range names {
		if val := os.Getenv(n); val != "" {
			return val
		}
	}
	return ""
}

func parseProxy(proxy string) (*url.URL, error) {
	if proxy == "" {
		return nil, nil
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil ||
		(proxyURL.Scheme != "http" &&
			proxyURL.Scheme != "https" &&
			proxyURL.Scheme != "socks5") {
		// proxy was bogus. Try prepending "http://" to it and
		// see if that parses correctly. If not, we fall
		// through and complain about the original one.
		if proxyURL, err := url.Parse("http://" + proxy); err == nil {
			return proxyURL, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}
	return proxyURL, nil
}

func doCheckProxy(scheme string) error {
	proxyScheme := fmt.Sprintf("%s_proxy", scheme)
	rawProxyURL := getEnvAny(strings.ToUpper(proxyScheme), strings.ToLower(proxyScheme))
	proxyURL, err := parseProxy(rawProxyURL)

	if err != nil {
		color.Red("\nA proxy has been set, but its url is invalid !\n\n")
		fmt.Printf("%s: %s", rawProxyURL, err)
		return fmt.Errorf("invalid %s value", strings.ToUpper(proxyScheme))
	}
	if proxyURL == nil {
		return nil
	}

	// Hide login/password
	if proxyURL.User != nil {
		pass, hasPass := proxyURL.User.Password()
		if hasPass {
			rawProxyURL = strings.Replace(rawProxyURL, pass, "****", -1)
		}
	}
	log.Info().Msg(fmt.Sprintf("Using %s proxy: %s", strings.ToUpper(scheme), rawProxyURL))
	return nil
}

// CheckProxy checks that the HTTP_PROXY is valid if it exists.
func CheckProxy() error {
	var errs []error
	if err := doCheckProxy("http"); err != nil {
		errs = append(errs, err)
	}
	if err := doCheckProxy("https"); err != nil {
		errs = append(errs, err)
	}
	if len(errs) != 0 {
		return fmt.Errorf("proxy setup has %d error(s)", len(errs))
	}
	return nil
}
