package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type DockerRegistry struct {
	HTTPClient *retryablehttp.Client
	URL        string
	Username   string
	AccessKey  string
}

type AuthToken struct {
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
}

func NewDockerRegistry(url, username, accessKey string, timeout time.Duration) DockerRegistry {
	return DockerRegistry{
		HTTPClient: NewRetryableClient(timeout),
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

func (c *DockerRegistry) Login(ctx context.Context, repo string) (AuthToken, error) {
	url := fmt.Sprintf("%s/v1alpha1/hosted/container-registry/%s/authorization-token", c.URL, repo)

	var authToken AuthToken
	req, err := NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return authToken, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return authToken, err
	}

	resp, err := c.HTTPClient.Do(r)
	if err != nil {
		return authToken, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return authToken, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&authToken); err != nil {
		return authToken, err
	}
	return authToken, nil
}
