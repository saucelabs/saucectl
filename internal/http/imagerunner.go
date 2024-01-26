package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/imagerunner"
)

type ImageRunner struct {
	Client            *retryablehttp.Client
	URL               string
	Creds             iam.Credentials
	AsyncEventManager imagerunner.AsyncEventManagerI
	eventLogger       zerolog.Logger
}

type AuthToken struct {
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
}

func NewImageRunner(url string, creds iam.Credentials, timeout time.Duration,
	asyncEventManager imagerunner.AsyncEventManagerI) ImageRunner {
	eventLogger := zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stdout,
		PartsOrder: []string{
			zerolog.MessageFieldName,
		},
		FormatLevel: func(i interface{}) string {
			return color.New(color.FgGreen).Sprint("[LOGS]")
		},
	})
	return ImageRunner{
		Client:            NewRetryableClient(timeout),
		URL:               url,
		Creds:             creds,
		AsyncEventManager: asyncEventManager,
		eventLogger:       eventLogger,
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

func (c *ImageRunner) getWebSocketURL() (string, error) {
	wsURL, err := url.Parse(c.URL)
	if err != nil {
		return "", err
	}
	if wsURL.Scheme == "https" {
		wsURL.Scheme = "wss"
	}
	if wsURL.Scheme == "http" {
		wsURL.Scheme = "ws"
	}
	if os.Getenv("SO_ASYNCEVENT_PORT") != "" {
		wsURL.Host = fmt.Sprintf("%s:%s", wsURL.Hostname(), os.Getenv("SO_ASYNCEVENT_PORT"))
	}
	return wsURL.String(), nil
}

func (c *ImageRunner) OpenAsyncEventsWebsocket(ctx context.Context, id string, lastseq string, nowait bool) (*websocket.Conn, error) {
	// dummy request so that we build basic auth header consistently
	dummyURL := fmt.Sprintf("%s/v1alpha1/hosted/async/image/runners/%s/events", c.URL, id)
	req, err := http.NewRequest("GET", dummyURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Creds.Username, c.Creds.AccessKey)

	websocketURL, err := c.getWebSocketURL()
	if err != nil {
		return nil, err
	}

	// build query string
	var queryParts []string
	if lastseq != "" {
		queryParts = append(queryParts, fmt.Sprintf("lastseq=%s", lastseq))
	}
	if nowait {
		queryParts = append(queryParts, "nowait=true")
	}
	query := ""
	if len(queryParts) > 0 {
		query = "?" + strings.Join(queryParts, "&")
	}

	url := fmt.Sprintf("%s/v1alpha1/hosted/async/image/runners/%s/events%s", websocketURL, id, query)
	headers := http.Header{}
	headers.Add("Authorization", req.Header.Get("Authorization"))
	dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: true,
	}
	ws, resp, err := dialer.Dial(url, headers)
	if resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden {
		return nil, imagerunner.AsyncEventFatalError{
			Err: errors.New(resp.Status),
		}
	}
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (c *ImageRunner) OpenAsyncEventsTransport(ctx context.Context, id string, lastseq string, nowait bool) (imagerunner.AsyncEventTransportI, error) {
	ws, err := c.OpenAsyncEventsWebsocket(ctx, id, lastseq, nowait)
	if err != nil {
		if _, ok := err.(imagerunner.AsyncEventFatalError); ok {
			return nil, err
		}
		return nil, imagerunner.AsyncEventSetupError{
			Err: err,
		}
	}
	return imagerunner.NewWebsocketAsyncEventTransport(ws), nil
}

func (c *ImageRunner) HandleAsyncEvents(ctx context.Context, id string, nowait bool) error {
	delay := 3 * time.Second
	var lastseq = ""
	var hasMoreLines bool
	var err error
	setupErrorCount := 0
	maxSetupErrors := 3
	for {
		if setupErrorCount >= maxSetupErrors {
			log.Info().Msgf("Could not setup Log streaming after %d attempts, disabling it.", maxSetupErrors)
			return imagerunner.AsyncEventSetupError{}
		}
		hasMoreLines, lastseq, err = c.handleAsyncEventsOneshot(ctx, id, lastseq, nowait)
		if errors.Is(err, context.Canceled) {
			return err
		}
		if _, ok := err.(imagerunner.AsyncEventFatalError); ok {
			return err
		}
		if !hasMoreLines {
			return nil
		}
		if wrappedErr, ok := err.(imagerunner.AsyncEventSetupError); ok {
			setupErrorCount++
			err = wrappedErr.Err
		} else {
			setupErrorCount = 0
		}
		log.Info().Err(err).Msgf("Log streaming issue. Retrying in %s...", delay)
		time.Sleep(delay)
	}
}

func (c *ImageRunner) handleAsyncEventsOneshot(ctx context.Context, id string, lastseq string, nowait bool) (bool, string, error) {
	transport, err := c.OpenAsyncEventsTransport(ctx, id, lastseq, nowait)
	if err != nil {
		return true, lastseq, err
	}
	if transport == nil {
		return true, lastseq, nil
	}

	defer transport.Close()

	// the first message is expected to be a ping
	readMessage, err := transport.ReadMessage()
	if err != nil {
		return true, lastseq, err
	}
	if readMessage == "" {
		return true, lastseq, errors.New("empty message")
	}
	event, err := c.AsyncEventManager.ParseEvent(readMessage)
	if err != nil {
		return true, lastseq, err
	}
	if event.Type == "com.saucelabs.so.v1.ping" {
		log.Info().Msg("Streaming logs...")
	} else {
		return true, lastseq, errors.New("first message is not a ping")
	}

	for {
		select {
		case <-ctx.Done():
			return false, lastseq, ctx.Err()
		default:
			readMessage, err := transport.ReadMessage()
			if err != nil {
				if nowait && strings.Contains(err.Error(), "close") {
					return false, lastseq, nil
				}
				return true, lastseq, err
			}
			if readMessage == "" {
				return true, lastseq, errors.New("empty message")
			}

			event, err := c.AsyncEventManager.ParseEvent(readMessage)
			if err != nil {
				return true, lastseq, err
			}
			switch event.Type {
			case "com.saucelabs.so.v1.ping":
			case "com.saucelabs.so.v1.log":
				if event.LineSequence != "" {
					lastseq = event.LineSequence
				}
				c.eventLogger.Info().Msgf("%s %s",
					color.New(color.FgCyan).Sprint(event.Data["containerName"]),
					event.Data["line"])
				c.AsyncEventManager.TrackLog()
			default:
				err := errors.New("unknown event type")
				log.Err(err).Msgf("unknown even type: %s", event.Type)
			}
		}
	}
}

func (c *ImageRunner) FetchLiveLogs(ctx context.Context, id string) error {
	err := c.HandleAsyncEvents(ctx, id, true)
	return err
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
