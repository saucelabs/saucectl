package testcomposer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

var (
	// ErrServerError is returned when the server was not able to correctly handle our request (status code >= 500).
	ErrServerError = errors.New(msg.InternalServerError)
	// ErrJobNotFound is returned when the requested job was not found.
	ErrJobNotFound = errors.New(msg.JobNotFound)
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
	Name       string     `json:"name"`
	Deprecated bool       `json:"deprecated"`
	Version    string     `json:"version"`
	Runner     runner     `json:"runner"`
	Platforms  []platform `json:"platforms"`
}

// TokenResponse represents the response body for slack token.
type TokenResponse struct {
	Token string `json:"token"`
}

type platform struct {
	Name     string
	Browsers []string
}

type runner struct {
	CloudRunnerVersion string `json:"cloudRunnerVersion"`
	DockerImage        string `json:"dockerImage"`
	GitRelease         string `json:"gitRelease"`
}

// GetSlackToken gets slack token.
func (c *Client) GetSlackToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/users/%s/settings/slack", c.URL, c.Credentials.Username)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	var resp TokenResponse
	if err := c.doJSONResponse(req, 200, &resp); err != nil {
		return "", err
	}

	return resp.Token, nil
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
		IsRDC bool
	}{}
	err = json.Unmarshal(body, &j)
	if err != nil {
		return
	}

	return j.JobID, j.IsRDC, nil
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
		Deprecated:         resp.Deprecated,
		DockerImage:        resp.Runner.DockerImage,
		GitRelease:         resp.Runner.GitRelease,
		CloudRunnerVersion: resp.Runner.CloudRunnerVersion,
	}

	return m, nil
}

func createUploadAssetRequest(ctx context.Context, url, username, accessKey, jobID, fileName, contentType string, content []byte) (*http.Request, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", fileName))
	h.Set("Content-Type", contentType)
	wr, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err = wr.Write(content); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}

	req, err := requesth.NewWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/v1/testcomposer/jobs/%s/assets", url, jobID), &b)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, accessKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

type assetsUploadResponse struct {
	Uploaded []string `json:"uploaded"`
	Errors   []string `json:"errors,omitempty"`
}

func doRequestAsset(httpClient *http.Client, request *http.Request) error {
	resp, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return ErrServerError
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrJobNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("assets upload request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var assetsResponse assetsUploadResponse
	if err = json.NewDecoder(resp.Body).Decode(&assetsResponse); err != nil {
		return err
	}
	if len(assetsResponse.Errors) > 0 {
		return fmt.Errorf("upload failed: %v", strings.Join(assetsResponse.Errors, ","))
	}
	return nil
}

// UploadAsset uploads an asset to the specified jobID.
func (c *Client) UploadAsset(jobID string, fileName string, contentType string, content []byte) error {
	request, err := createUploadAssetRequest(context.Background(), c.URL, c.Credentials.Username, c.Credentials.AccessKey, jobID, fileName, contentType, content)
	if err != nil {
		return err
	}
	return doRequestAsset(c.HTTPClient, request)
}

// Frameworks returns the list of available frameworks.
func (c *Client) Frameworks(ctx context.Context) ([]framework.Framework, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/frameworks", c.URL)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []framework.Framework{}, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	var resp []framework.Framework
	if err = c.doJSONResponse(req, 200, &resp); err != nil {
		return []framework.Framework{}, err
	}
	return resp, nil
}

// Versions return the list of available versions for a specific framework and region.
func (c *Client) Versions(ctx context.Context, frameworkName string) ([]framework.Metadata, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/frameworks/%s/versions", c.URL, frameworkName)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []framework.Metadata{}, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	var resp []FrameworkResponse
	if err = c.doJSONResponse(req, 200, &resp); err != nil {
		return []framework.Metadata{}, err
	}

	var frameworks []framework.Metadata
	for _, f := range resp {
		var platforms []framework.Platform
		for _, p := range f.Platforms {
			platforms = append(platforms, framework.Platform{
				PlatformName: p.Name,
				BrowserNames: p.Browsers,
			})
		}
		frameworks = append(frameworks, framework.Metadata{
			FrameworkName:    f.Name,
			FrameworkVersion: f.Version,
			Deprecated:       f.Deprecated,
			DockerImage:      f.Runner.DockerImage,
			GitRelease:       f.Runner.GitRelease,
			Platforms:        platforms,
		})
	}
	return frameworks, nil
}
