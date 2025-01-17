package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/region"
)

func NewBuildService(
	r region.Region, username, accessKey string, timeout time.Duration,
) BuildService {
	return BuildService{
		Client:    NewRetryableClient(timeout),
		URL:       r.APIBaseURL(),
		AppURL:    r.AppBaseURL(),
		Username:  username,
		AccessKey: accessKey,
	}
}

type BuildService struct {
	Client    *retryablehttp.Client
	AppURL    string
	URL       string
	Username  string
	AccessKey string
}

func (c *BuildService) FindBuild(
	ctx context.Context, jobID string, realDevice bool,
) (build.Build, error) {
	src := "vdc"
	if realDevice {
		src = "rdc"
	}

	req, err := NewRetryableRequestWithContext(
		ctx, http.MethodGet, fmt.Sprintf(
			"%s/v2/builds/%s/jobs/%s/build/", c.URL, src, jobID,
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

	var b build.Build
	if err = json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return build.Build{}, err
	}

	b.URL = fmt.Sprintf(
		"%s/builds/%s/%s", c.AppURL, src,
		b.ID,
	)

	return b, err
}
