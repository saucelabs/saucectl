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

func (c *BuildService) ListBuilds(
	ctx context.Context,
	opts build.ListBuildsOptions,
) ([]build.Build, error) {
	req, err := NewRetryableRequestWithContext(
		ctx, http.MethodGet, fmt.Sprintf(
			"%s/v2/builds/%s", c.URL, opts.Source,
		), nil,
	)

	if err != nil {
		return []build.Build{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return []build.Build{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return []build.Build{}, fmt.Errorf(
			"unexpected statusCode: %v", resp.StatusCode,
		)
	}

	var b build.BuildResponse
	if err = json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return []build.Build{}, err
	}

	for _, b := range b.Builds {
		b.URL = fmt.Sprintf(
			"%s/builds/%s/%s", c.AppURL, opts.Source,
			b.ID,
		)
	}

	return b.Builds, err
}

func (c *BuildService) GetBuild(
	ctx context.Context,
	opts build.GetBuildOptions,
) (build.Build, error) {

	var url string
	if opts.ByJob {
		url = fmt.Sprintf(
			"%s/v2/builds/%s/jobs/%s/build/", c.URL, opts.Source, opts.ID,
		)
	} else {
		url = fmt.Sprintf(
			"%s/v2/builds/%s/%s/", c.URL, opts.Source, opts.ID,
		)
	}

	req, err := NewRetryableRequestWithContext(
		ctx, http.MethodGet, url, nil,
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
		"%s/builds/%s/%s", c.AppURL, opts.Source,
		b.ID,
	)

	return b, err
}
