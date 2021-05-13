package testcomposer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/requesth"
	"io"
	"net/http"
	"strings"
)

// Client service
type Client struct {
	HTTPClient  *http.Client
	URL         string // e.g.) https://api.<region>.saucelabs.net
	Credentials credentials.Credentials
}

// Job represents the sauce labs test job.
type Job struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
}

// FrameworkResponse represents the response body for framework information.
type FrameworkResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Runner  runner `json:"runner"`
}

type runner struct {
	CloudRunnerVersion string `json:"cloudRunnerVersion"`
	DockerImage        string `json:"dockerImage"`
	GitRelease         string `json:"gitRelease"`
}

// StartJob creates a new job in Sauce Labs.
func (c *Client) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
	url := fmt.Sprintf("%s/v1/testcomposer/jobs", c.URL)

	opts.User = c.Credentials.Username
	opts.AccessKey = c.Credentials.AccessKey

	var b bytes.Buffer
	err = json.NewEncoder(&b).Encode(opts)
	if err != nil {
		return
	}
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return
	}
	req.SetBasicAuth(opts.User, opts.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode >= 300 {
		err = fmt.Errorf("job start failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, strings.TrimSpace(string(body)))
		return "", false, err
	}

	j := struct {
		JobID string
		isRDC bool
	}{}
	err = json.Unmarshal(body, &j)
	if err != nil {
		return
	}

	return j.JobID, j.isRDC, nil
}

func (c *Client) newJSONRequest(ctx context.Context, url, method string, payload interface{}) (*http.Request, error) {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(payload); err != nil {
		return nil, err
	}

	req, err := requesth.NewWithContext(ctx, method, url, &b)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	req.Header.Set("Content-Type", "application/json")

	return req, err
}

func (c *Client) doJSONResponse(req *http.Request, expectStatus int, v interface{}) error {
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != expectStatus {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("unexpected status '%d' from test-composer: %s", res.StatusCode, body)
	}

	return json.NewDecoder(res.Body).Decode(v)
}

// Search returns metadata for the given search options opts.
func (c *Client) Search(ctx context.Context, opts framework.SearchOptions) (framework.Metadata, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/frameworks/%s", c.URL, opts.Name)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return framework.Metadata{}, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	q := req.URL.Query()
	q.Add("version", opts.FrameworkVersion)
	req.URL.RawQuery = q.Encode()

	var resp FrameworkResponse
	if err := c.doJSONResponse(req, 200, &resp); err != nil {
		return framework.Metadata{}, err
	}

	m := framework.Metadata{
		FrameworkName:      resp.Name,
		FrameworkVersion:   resp.Version,
		CloudRunnerVersion: resp.Runner.CloudRunnerVersion,
		DockerImage:        resp.Runner.DockerImage,
		GitRelease:         resp.Runner.GitRelease,
	}

	return m, nil
}
