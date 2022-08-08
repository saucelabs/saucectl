package user

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// Client service
type Client struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

// User represents user data structure
type User struct {
	ID           string       `json:"id"`
	Groups       []Group      `json:"groups"`
	Organization Organization `json:"organization"`
}

// Group represents the group that the user belongs to
type Group struct {
	ID string `json:"id"`
}

// Organization represents the organization that the user belongs to
type Organization struct {
	ID string `json:"id"`
}

// Get user data
func (c *Client) Get(ctx context.Context) (User, error) {
	url := fmt.Sprintf("%s/team-management/v1/users/me", c.URL)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return User{}, err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return User{}, err
	}
	user := User{}
	err = json.Unmarshal(body, &user)
	if err != nil {
		return user, err
	}
	return user, nil
}
