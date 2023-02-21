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

func (c *UserService) GetUser(ctx context.Context) (iam.User, error) {
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
