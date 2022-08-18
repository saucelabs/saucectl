package apitesting

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/requesth"
)

type Client struct {
	HTTPClient *http.Client
	URL        string
	Username   string
	AccessKey  string
}

type RunSyncResponse struct {
	ID            string  `json:"id"`
	FailuresCount int     `json:"failuresCount"`
	Project       Project `json:"project"`
}

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func New(url string, username string, accessKey string, timeout time.Duration) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

func (c *Client) RunAllSync(ctx context.Context, hookId string, format string, buildId string) ([]RunSyncResponse, error) {
	log.Info().Str("hookId", hookId).Msg("Running project")

	var runResp []RunSyncResponse

	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/_run-all-sync?format=%s", c.URL, hookId, format)
	log.Info().Str("username", c.Username).Msgf("api url: %s", url)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return runResp, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return runResp, err
	}
	// TODO: Check response code?
	if resp.StatusCode != 200 {
		return runResp, fmt.Errorf("Got a non-200 response: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return runResp, err
	}

	
	err = json.Unmarshal(body, &runResp)
	if err != nil {
		return runResp, err
	}

	return runResp, nil
}
