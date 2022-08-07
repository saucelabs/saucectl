package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/requesth"
	"github.com/saucelabs/saucectl/internal/user"
)

//https://api.us-west-1.saucelabs.com/v2/insights/vdc/test-cases?start=1659051369&since=1659051369&end=1659656169&until=1659656169&org_id=7fb25570b4064716b9b6daae1a846790&limit=200&offset=0&sort=desc&sort_by=fail_rate

type Client struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

type TestHistory struct {
	TestCases []TestCase `json:"test_cases"`
}

type TestCase struct {
	Name     string  `json:"name"`
	FailRate float64 `json:"fail_rate"`
}

type LaunchBy string

const (
	LaunchByFailrate LaunchBy = "fail_rate"
)

func (c *Client) GetHistory(ctx context.Context, user user.User, launchBy LaunchBy) (TestHistory, error) {
	now := time.Now().Unix()
	start := time.Now().AddDate(0, 0, -1).Unix()
	url := fmt.Sprintf("%s/v2/insights/vdc/test-cases?user_id=%s&start=%d&since=%d&end=%d&until=%d&org_id=%s&limit=200&sort=desc&sort_by=%s",
		c.URL, user.ID, start, start, now, now, user.Organization.ID, launchBy)

	testHistory := TestHistory{}
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return testHistory, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return testHistory, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return testHistory, err
	}

	err = json.Unmarshal(body, &testHistory)
	if err != nil {
		return testHistory, err
	}
	return testHistory, nil
}
