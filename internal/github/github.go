package github

import (
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/cli/version"
	"github.com/saucelabs/saucectl/internal/requesth"
	"golang.org/x/mod/semver"
	"net/http"
	"strings"
)

// Client represents the Github HTTP API client.
type Client struct {
	HTTPClient *http.Client
	URL        string
}

type release struct {
	Name    string `json:"name"`
	TagName string `json:"tag_name"`
}

// HasUpdateAvailable returns the version number of latest available update if there is one.
func (c *Client) HasUpdateAvailable() (string, error) {
	req, err := requesth.New(http.MethodGet, fmt.Sprintf("%s/repos/saucelabs/saucectl/releases/latest", c.URL), nil)
	if err != nil {
		return "", err
	}

	r, err := c.executeRequest(req)
	if err != nil {
		return "", err
	}

	if isUpdateRequired(version.Version, r.TagName) {
		return r.TagName, nil
	}
	return "", nil
}

func (c *Client) executeRequest(req *http.Request) (release, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return release{}, nil
	}
	defer resp.Body.Close()

	var r release
	if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return release{}, nil
	}
	return r, nil
}

func isUpdateRequired(current, remote string) bool {
	currentV := current
	if !strings.HasPrefix(currentV, "v") {
		currentV = fmt.Sprintf("v%s", currentV)
	}
	remoteV := remote
	if !strings.HasPrefix(remoteV, "v") {
		remoteV = fmt.Sprintf("v%s", remoteV)
	}
	return semver.Compare(currentV, remoteV) < 0
}
