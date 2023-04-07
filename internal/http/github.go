package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// DefaultGitHub is a preconfigured instance of GitHub.
var DefaultGitHub = GitHub{
	HTTPClient: &http.Client{
		Timeout:   4 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	},
	URL: "https://api.github.com",
}

// GitHub represents the GitHub HTTP API client.
type GitHub struct {
	HTTPClient *http.Client
	URL        string
}

// IsUpdateAvailable returns the latest version if it's semantically higher than the given one.
func (c *GitHub) IsUpdateAvailable(version string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/repos/saucelabs/saucectl/releases/latest", c.URL), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()

	var r struct {
		Name    string `json:"name"`
		TagName string `json:"tag_name"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", nil
	}

	if c.isUpdateRequired(version, r.TagName) {
		return r.TagName, nil
	}
	return "", nil
}

func (c *GitHub) isUpdateRequired(currentVersion, githubVersion string) bool {
	currentV := currentVersion
	if !strings.HasPrefix(currentV, "v") {
		currentV = fmt.Sprintf("v%s", currentV)
	}
	githubV := githubVersion
	if !strings.HasPrefix(githubV, "v") {
		githubV = fmt.Sprintf("v%s", githubV)
	}
	return semver.Compare(currentV, githubV) < 0
}
