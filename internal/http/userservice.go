package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/iam"
)

type UserService struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

func NewUserService(url string, creds credentials.Credentials, timeout time.Duration) UserService {
	return UserService{
		HTTPClient:  &http.Client{Timeout: timeout},
		URL:         url,
		Credentials: creds,
	}
}

// GetConcurrency returns the concurrency settings for the current account.
func (c *UserService) Concurrency(ctx context.Context) (iam.Concurrency, error) {
	req, err := NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/rest/v1.2/users/%s/concurrency", c.URL, c.Credentials.Username), nil)
	if err != nil {
		return iam.Concurrency{}, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return iam.Concurrency{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return iam.Concurrency{}, fmt.Errorf("unexpected server response (%d): %s", resp.StatusCode, b)
	}

	var body struct {
		Concurrency iam.Concurrency `json:"concurrency"`
	}

	return body.Concurrency, json.NewDecoder(resp.Body).Decode(&body)
}

func (c *UserService) User(ctx context.Context) (iam.User, error) {
	url := fmt.Sprintf("%s/team-management/v1/users/me", c.URL)

	var user iam.User
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return user, err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return user, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return user, err
	}
	err = json.Unmarshal(body, &user)
	if err != nil {
		return user, err
	}
	return user, nil
}
