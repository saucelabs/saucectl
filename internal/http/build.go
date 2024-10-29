package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/build"
)

func NewBuildService(
	url, username, accessKey string, timeout time.Duration,
) BuildService {
	return BuildService{
		Client:    NewRetryableClient(timeout),
		URL:       url,
		Username:  username,
		AccessKey: accessKey,
	}
}

type BuildService struct {
	Client    *retryablehttp.Client
	URL       string
	Username  string
	AccessKey string
}

func (c *BuildService) FindBuild(
	ctx context.Context, jobID string, buildSource build.Source,
) (build.Build, error) {
	req, err := NewRetryableRequestWithContext(
		ctx, http.MethodGet, fmt.Sprintf(
			"%s/v2/builds/%s/jobs/%s/build/", c.URL, buildSource, jobID,
		), nil,
	)
	if err != nil {
		return build.Build{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return build.Build{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return build.Build{}, fmt.Errorf(
			"unexpected statusCode: %v", resp.StatusCode,
		)
	}

	var br build.Build
	return br, json.NewDecoder(resp.Body).Decode(&br)
}
