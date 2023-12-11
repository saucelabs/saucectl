package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/imagerunner"
)

type ImageRunner struct {
	Client *retryablehttp.Client
	URL    string
	Creds  iam.Credentials
}

type AuthToken struct {
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
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

	switch resp.StatusCode {
	case http.StatusCreated:
		return runner, json.Unmarshal(body, &runner)
	default:
		return runner, c.newServerError(resp.StatusCode, "runner start", body)
	}
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

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, ErrServerError
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed; unexpected response code: '%d', msg: '%v'", resp.StatusCode, string(body))
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

	if resp.StatusCode == http.StatusNotFound {
		return "", imagerunner.ErrResourceNotFound
	}
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

func (c *ImageRunner) getWebsocketURL() string {

	wsURL := c.URL
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	return wsURL
}

func (c *ImageRunner) OpenAsyncEventsWebsocket(ctx context.Context, id string) (*websocket.Conn, error) {
	// dummy request so that we build basic auth header consistently
	dummyURL := fmt.Sprintf("%s/v1alpha1/hosted/async/image/runners/%s/events", c.URL, id)
	req, err := http.NewRequest("GET", dummyURL, nil)
	if err != nil {
		panic(err)
	}
	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)

	url := fmt.Sprintf("%s/v1alpha1/hosted/async/image/runners/%s/events", c.getWebsocketURL(), id)
	headers := http.Header{}
	headers.Add("Authorization", req.Header.Get("Authorization"))
	ws, resp, err := websocket.DefaultDialer.Dial(
		url, headers)
	if err != nil {
		if resp != nil {
			log.Error().Err(err).Int("http status", resp.StatusCode).Msg("Could not open async events websocket")
		} else {
			log.Error().Err(err).Msg("Could not open async events websocket")
		}
		return nil, err
	}
	return ws, nil
}

func (c *ImageRunner) OpenAsyncEventsSSE(ctx context.Context, id string) (*http.Response, error) {
	url := fmt.Sprintf("%s/v1alpha1/hosted/async/image/runners/%s/events", c.URL, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")
	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected server response (%d)", resp.StatusCode)
	}
	return resp, nil
}

func (c *ImageRunner) OpenAsyncEventsTransport(ctx context.Context, id string) (imagerunner.AsyncEventTransportI, error) {
	if os.Getenv("LIVELOGS") == "sse" {
		resp, err := c.OpenAsyncEventsSSE(ctx, id)
		return imagerunner.NewSseAsyncEventTransport(resp), err
	}

	ws, err := c.OpenAsyncEventsWebsocket(ctx, id)
	return imagerunner.NewWebsocketAsyncEventTransport(ws), err
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

func (c *ImageRunner) newServerError(status int, short string, body []byte) error {
	var se imagerunner.ServerError
	err := json.Unmarshal(body, &se)
	if err != nil || (se.Code == "" && se.Msg == "") {
		// If the body doesn't conform to the server error format, just return
		// the raw body.
		se.Code = "ERR_SERVER_ERROR"
		se.Msg = string(body)
	}
	se.HTTPStatus = status
	se.Short = short

	return &se
}

func (c *ImageRunner) RegistryLogin(ctx context.Context, repo string) (AuthToken, error) {
	url := fmt.Sprintf("%s/v1alpha1/hosted/container-registry/%s/authorization-token", c.URL, repo)

	var authToken AuthToken
	req, err := NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return authToken, err
	}
	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)

	r, err := retryablehttp.FromRequest(req)
	if err != nil {
		return authToken, err
	}

	resp, err := c.Client.Do(r)
	if err != nil {
		return authToken, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return authToken, err
	}
	if resp.StatusCode != 200 {
		return authToken, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(data))
	}

	err = json.Unmarshal(data, &authToken)
	return authToken, err
}
