package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// Client service
type Client struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

var LaunchOptions = map[config.LaunchOrder]string{
	config.LaunchOrderFailRate: "fail_rate",
}

func New(url string, creds credentials.Credentials, timeout time.Duration) Client {
	return Client{
		HTTPClient:  &http.Client{Timeout: timeout},
		URL:         url,
		Credentials: creds,
	}
}

// GetHistory returns job history from insights
func (c *Client) GetHistory(ctx context.Context, user iam.User, launchOrder config.LaunchOrder) (JobHistory, error) {
	start := time.Now().AddDate(0, 0, -7).Unix()
	now := time.Now().Unix()

	var jobHistory JobHistory
	url := fmt.Sprintf("%s/v2/insights/vdc/test-cases", c.URL)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return jobHistory, err
	}

	q := req.URL.Query()
	queries := map[string]string{
		"user_id": user.ID,
		"org_id":  user.Organization.ID,
		"start":   strconv.FormatInt(start, 10),
		"since":   strconv.FormatInt(start, 10),
		"end":     strconv.FormatInt(now, 10),
		"until":   strconv.FormatInt(now, 10),
		"limit":   "200",
		"offset":  "0",
		"sort_by": string(launchOrder),
	}
	for k, v := range queries {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return jobHistory, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return jobHistory, err
	}

	err = json.Unmarshal(body, &jobHistory)
	if err != nil {
		return jobHistory, err
	}
	return jobHistory, nil
}
