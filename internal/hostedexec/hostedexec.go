package hostedexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// TODO Caution: Final states are not confirmed yet.
// The different states that a runner can be in.
const (
	StatePending   = "Pending"
	StateRunning   = "Running"
	StateSucceeded = "Succeeded"
	StateCancelled = "Cancelled"
	StateFailed    = "Failed"
)

// DoneStates represents states that a runner doesn't transition out of, i.e. once the runner is in one of these states,
// it's done.
var DoneStates = []string{StateSucceeded, StateCancelled, StateFailed}

type Client struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

type RunnerSpec struct {
	Container  Container         `json:"container,omitempty"`
	EntryPoint string            `json:"entrypoint,omitempty"`
	Env        []EnvItem         `json:"env,omitempty"`
	Files      []FileData        `json:"files,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	// Artifacts  []string
}

type Container struct {
	Name string `json:"name,omitempty"`
	Auth Auth   `json:"auth,omitempty"`
}

type Auth struct {
	User  string `json:"user,omitempty"`
	Token string `json:"token,omitempty"`
}

type EnvItem struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type FileData struct {
	Path string `json:"path,omitempty"`
	Data string `json:"data,omitempty"`
}

type Runner struct {
	ID                string `json:"id,omitempty"`
	Status            string `json:"status,omitempty"`
	Image             string `json:"image,omitempty"`
	CreationTime      int64  `json:"creation_time,omitempty"`
	TerminationTime   int64  `json:"termination_time,omitempty"`
	TerminationReason string `json:"termination_reason,omitempty"`
}

type RunnerDetails struct {
	Runner
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Service interface {
	TriggerRun(context.Context, RunnerSpec) (Runner, error)
	GetRun(ctx context.Context, id string) (RunnerDetails, error)
	StopRun(ctx context.Context, id string) error
}

func New(url string, creds credentials.Credentials, timeout time.Duration) Client {
	return Client{
		HTTPClient:  &http.Client{Timeout: timeout},
		URL:         url,
		Credentials: creds,
	}
}
func (c *Client) TriggerRun(ctx context.Context, spec RunnerSpec) (Runner, error) {
	var runner Runner
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

func (c *Client) GetRun(ctx context.Context, id string) (RunnerDetails, error) {
	var r RunnerDetails
	url := fmt.Sprintf("%s/v1alpha1/hosted/image/runners/%s", c.URL, id)

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
	err = json.Unmarshal(body, &r)

	return r, nil
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

// Done returns true if the runner status is one of DoneStates. False otherwise.
func Done(status string) bool {
	for _, s := range DoneStates {
		if s == status {
			return true
		}
	}

	return false
}
