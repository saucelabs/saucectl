package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/imagerunner"
)

type ImageRunner struct {
	Client *retryablehttp.Client
	URL    string
	Creds  iam.Credentials
}

func NewImageRunner(url string, creds iam.Credentials, timeout time.Duration) ImageRunner {
	return ImageRunner{
		Client: NewRetryableClient(timeout),
		URL:    url,
		Creds:  creds,
	}
}
func (c *ImageRunner) TriggerRun(ctx context.Context, spec imagerunner.RunnerSpec) (imagerunner.Runner, error) {
	var runner imagerunner.Runner
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners", c.URL)

	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(spec)
	if err != nil {
		return runner, err
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return runner, err
	}

	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return runner, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return runner, err
	}

	if resp.StatusCode != http.StatusCreated {
		return runner, fmt.Errorf("runner start failed (%d): %s", resp.StatusCode, body)
	}

	return runner, json.Unmarshal(body, &runner)
}

func (c *ImageRunner) GetStatus(ctx context.Context, id string) (imagerunner.Runner, error) {
	var r imagerunner.Runner
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s/status", c.URL, id)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return r, err
	}

	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return r, err
	}

	if resp.StatusCode != http.StatusOK {
		return r, fmt.Errorf("runner status retrieval failed (%d): %s", resp.StatusCode, body)
	}

	return r, json.Unmarshal(body, &r)
}

func (c *ImageRunner) StopRun(ctx context.Context, runID string) error {
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s", c.URL, runID)

	req, err := NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *ImageRunner) DownloadArtifacts(ctx context.Context, id string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s/artifacts/url", c.URL, id)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected server response (%d): %s", resp.StatusCode, b)
	}

	type response struct {
		URL string `json:"url"`
	}

	var urlLink response
	if err = json.NewDecoder(resp.Body).Decode(&urlLink); err != nil {
		return nil, fmt.Errorf("failed to decode server response: %w", err)
	}

	req, err = NewRetryableRequestWithContext(ctx, http.MethodGet, urlLink.URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err = c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (c *ImageRunner) GetLogs(ctx context.Context, id string) (string, error) {
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s/logs/url", c.URL, id)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected server response (%d): %s", resp.StatusCode, b)
	}

	var urlResponse struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&urlResponse); err != nil {
		return "", fmt.Errorf("failed to decode server response: %w", err)
	}

	return c.doGetStr(ctx, urlResponse.URL)
}

func (c *ImageRunner) doGetStr(ctx context.Context, url string) (string, error) {
	urlReq, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.Client.Do(urlReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", imagerunner.ErrResourceNotFound
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected server response (%d): %s", resp.StatusCode, b)
	}

	builder := &strings.Builder{}
	if _, err := io.Copy(builder, resp.Body); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	return builder.String(), nil
}
