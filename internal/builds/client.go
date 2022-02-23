package builds

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/requesth"
)

type Client struct {
	HTTPClient *retryablehttp.Client
	URL        string
	Username   string
	AccessKey  string
}

// BuildSource defines the type of test device associated with the job and build
type BuildSource string

const (
	// VDC refers to jobs executed on virtual devices (e.g. VMs, emulators, simulators)
	VDC BuildSource = "vdc"
	// RDC refers to jobs executed on real mobile devices
	RDC             = "rdc"
)

// buildResponse is the response body returned from /v2/builds/{buildSource}/jobs/{jobID}/build/
type buildResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func New(url, username, accessKey string, timeout time.Duration) Client {
	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient = &http.Client{Timeout: timeout}
	httpClient.Logger = nil

	return Client{
		HTTPClient: httpClient,
		URL:        url,
		Username:   username,
		AccessKey:  accessKey,
	}
}

func (c *Client) GetBuildForJob(ctx context.Context, job job.Job) (string, error) {
	jobID := job.ID
	var buildSource BuildSource
	if job.IsRDC {
		buildSource = RDC
	} else {
		buildSource = VDC
	}

	req, err := requesth.NewWithContext(ctx, http.MethodGet, fmt.Sprintf("/v2/builds/%s/jobs/%s/build/", buildSource, jobID), nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTPClient.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// TODO: Handle non-OK status
	}

	return "", nil
}
