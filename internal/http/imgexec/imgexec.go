package imgexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/requesth"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Client struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

func New(url string, creds credentials.Credentials, timeout time.Duration) Client {
	return Client{
		HTTPClient:  &http.Client{Timeout: timeout},
		URL:         url,
		Credentials: creds,
	}
}
func (c *Client) TriggerRun(ctx context.Context, spec imagerunner.RunnerSpec) (imagerunner.Runner, error) {
	var runner imagerunner.Runner
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners", c.URL)

	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(spec)
	if err != nil {
		return runner, err
	}
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return runner, err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
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

func (c *Client) GetStatus(ctx context.Context, id string) (imagerunner.Runner, error) {
	var r imagerunner.Runner
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s/status", c.URL, id)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return r, err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
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

func (c *Client) StopRun(ctx context.Context, runID string) error {
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s", c.URL, runID)

	req, err := requesth.NewWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) ListArtifacts(ctx context.Context, id string) ([]string, error) {
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s/artifacts", c.URL, id)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []string{}, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return []string{}, fmt.Errorf("unexpected server response (%d): %s", resp.StatusCode, b)
	}

	// TODO response type is not confirmed yet
	type response struct {
		Artifacts []string `json:"artifacts"`
	}

	var listResponse response
	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		return []string{}, fmt.Errorf("failed to decode server response: %w", err)
	}

	return listResponse.Artifacts, nil
}

func (c *Client) DownloadArtifact(ctx context.Context, id, name, dir string) error {
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s/artifacts/single/%s/download", c.URL, id, name)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected server response (%d): %s", resp.StatusCode, b)
	}

	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	if _, err := f.ReadFrom(resp.Body); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	return nil
}
