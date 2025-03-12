package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	q := req.URL.Query()
	queries := map[string]string{
		"offset": strconv.Itoa(opts.Page * opts.Size),
		"limit":  strconv.Itoa(opts.Size),
		"status": string(opts.Status),
		"name":   opts.Name,
	}
	for k, v := range queries {
		if v != "" {
			q.Add(k, v)
		}
	}
	req.URL.RawQuery = q.Encode()

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

	var br build.BuildResponse
	if err = json.NewDecoder(resp.Body).Decode(&br); err != nil {
		return []build.Build{}, err
	}

	for i := 0; i < len(br.Builds); i++ {
		b := &br.Builds[i]
		b.URL = fmt.Sprintf(
			"%s/builds/%s/%s", c.AppURL, opts.Source, b.ID,
		)
	}

	return br.Builds, err
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
