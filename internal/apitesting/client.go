package apitesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

type Client struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

type TestResult struct {
	EventID       string  `json:"id,omitempty"`
	FailuresCount int     `json:"failuresCount,omitempty"`
	Project       Project `json:"project,omitempty"`
	Test          Test    `json:"test,omitempty"`
	Async         bool    `json:"-,omitempty"`
}

type Test struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Project struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func New(url string, username string, accessKey string, timeout time.Duration) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

func (c *Client) GetProject(ctx context.Context, hookId string) (Project, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s", c.URL, hookId)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Project{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)

	if resp.StatusCode >= http.StatusInternalServerError {
		return Project{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Project{}, fmt.Errorf("Failed to fetch project details; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	project := Project{}
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return project, err
	}
	return project, nil
}

func (c *Client) composeURL(path string, buildId string, format string, tunnel config.Tunnel) string {
	// NOTE: API url is not user provided so skip error check
	url, _ := url.Parse(c.URL)
	url.Path = path

	query := url.Query()
	if buildId != "" {
		query.Set("buildId", buildId)
	}
	if format != "" {
		query.Set("format", format)
	}

	if tunnel.Name != "" {
		t := tunnel.Name
		if tunnel.Owner != "" {
			t = fmt.Sprintf("%s:%s", t, tunnel.Owner)
		}

		query.Set("tunnelId", t)
	}

	url.RawQuery = query.Encode()

	return url.String()
}
