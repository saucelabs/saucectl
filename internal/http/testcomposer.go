package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/iam"
)

// TestComposer service
type TestComposer struct {
	HTTPClient  *retryablehttp.Client
	URL         string // e.g. https://api.<region>.saucelabs.com
	Credentials iam.Credentials
}

// FrameworkResponse represents the response body for framework information.
type FrameworkResponse struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	EOLDate     time.Time `json:"eolDate"`
	RemovalDate time.Time `json:"removalDate"`
	Runner      runner    `json:"runner"`
	Platforms   []struct {
		Name     string
		Browsers []string
	} `json:"platforms"`
	BrowserDefaults map[string]string `json:"browserDefaults"`
	Runtimes        []string          `json:"runtimes"`
}

// TokenResponse represents the response body for slack token.
type TokenResponse struct {
	Token string `json:"token"`
}

type runner struct {
	CloudRunnerVersion string `json:"cloudRunnerVersion"`
	DockerImage        string `json:"dockerImage"`
	GitRelease         string `json:"gitRelease"`
}

type RuntimeResponse struct {
	Name     string    `json:"name"`
	Releases []Release `json:"releases"`
}

type Release struct {
	Version string    `json:"version"`
	Aliases []string  `json:"aliases"`
	EOLDate time.Time `json:"eolDate"`
	Default bool      `json:"default"`

	Extra map[string]string `json:"extra"`
}

func NewTestComposer(url string, creds iam.Credentials, timeout time.Duration) TestComposer {
	return TestComposer{
		HTTPClient:  NewRetryableClient(timeout),
		URL:         url,
		Credentials: creds,
	}
}

// GetSlackToken gets slack token.
func (c *TestComposer) GetSlackToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/users/%s/settings/slack", c.URL, c.Credentials.Username)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
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

func (c *TestComposer) doJSONResponse(req *retryablehttp.Request, expectStatus int, v interface{}) error {
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

// UploadAsset uploads an asset to the specified jobID.
func (c *TestComposer) UploadAsset(jobID string, realDevice bool, fileName string, contentType string, content []byte) error {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", fileName))
	h.Set("Content-Type", contentType)
	wr, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err = wr.Write(content); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}

	req, err := NewRetryableRequestWithContext(context.Background(), http.MethodPut,
		fmt.Sprintf("%s/v1/testcomposer/jobs/%s/assets", c.URL, jobID), &b)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
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

	var assetsResponse struct {
		Uploaded []string `json:"uploaded"`
		Errors   []string `json:"errors,omitempty"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&assetsResponse); err != nil {
		return err
	}
	if len(assetsResponse.Errors) > 0 {
		return fmt.Errorf("upload failed: %v", strings.Join(assetsResponse.Errors, ","))
	}
	return nil
}

// Frameworks returns the list of available frameworks.
func (c *TestComposer) Frameworks(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/v2/testcomposer/frameworks", c.URL)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []string{}, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	var resp []framework.Framework
	if err = c.doJSONResponse(req, 200, &resp); err != nil {
		return []string{}, err
	}
	return uniqFrameworkNameSet(resp), nil
}

// Versions return the list of available versions for a specific framework and region.
func (c *TestComposer) Versions(ctx context.Context, frameworkName string) ([]framework.Metadata, error) {
	url := fmt.Sprintf("%s/v2/testcomposer/frameworks?frameworkName=%s", c.URL, frameworkName)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
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
			FrameworkName:      f.Name,
			FrameworkVersion:   f.Version,
			EOLDate:            f.EOLDate,
			RemovalDate:        f.RemovalDate,
			DockerImage:        f.Runner.DockerImage,
			GitRelease:         f.Runner.GitRelease,
			Platforms:          platforms,
			CloudRunnerVersion: f.Runner.CloudRunnerVersion,
			BrowserDefaults:    f.BrowserDefaults,
			Runtimes:           f.Runtimes,
		})
	}
	return frameworks, nil
}

func uniqFrameworkNameSet(frameworks []framework.Framework) []string {
	var fws []string
	mp := map[string]bool{}

	for _, fw := range frameworks {
		_, present := mp[fw.Name]

		if !present {
			mp[fw.Name] = true
			fws = append(fws, fw.Name)
		}
	}
	fmt.Println("frameworks: ", fws)
	return fws
}

func (c *TestComposer) Runtimes(ctx context.Context) ([]framework.Runtime, error) {
	url := fmt.Sprintf("%s/v1/testcomposer/runtimes", c.URL)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	var resp []RuntimeResponse
	if err = c.doJSONResponse(req, 200, &resp); err != nil {
		return nil, err
	}

	var runtimes []framework.Runtime
	for _, rt := range resp {
		name := rt.Name
		for _, r := range rt.Releases {
			runtimes = append(runtimes, framework.Runtime{
				RuntimeName:    name,
				RuntimeVersion: r.Version,
				RuntimeAlias:   r.Aliases,
				EOLDate:        r.EOLDate,
				Extra:          r.Extra,
			})
		}
	}

	return runtimes, nil
}
