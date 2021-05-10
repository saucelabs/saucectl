package rdc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/internal/requesth"
	"net/http"
	"time"
)

// Client http client.
type Client struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

type organizationResponse struct {
	Maximum int `json:"maximum,omitempty"`
}

type concurrencyResponse struct {
	Organization organizationResponse `json:"organization,omitempty"`
}

// New creates a new client.
func New(url, username, accessKey string, timeout time.Duration) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

// ReadAllowedCCY returns the allowed (max) concurrency for the current account.
func (c *Client) ReadAllowedCCY(ctx context.Context) (int, error) {
	req, err := requesth.NewWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/rdc/concurrency", c.URL), nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("unexpected statusCode: %v", resp.StatusCode)
	}

	var cr concurrencyResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return 0, err
	}

	return cr.Organization.Maximum, nil
}