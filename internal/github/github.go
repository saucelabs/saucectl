package github

import (
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/cli/version"
	"github.com/saucelabs/saucectl/internal/requesth"
	"golang.org/x/mod/semver"
	"net/http"
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
	return semver.Compare(current, remote) < 0
}
