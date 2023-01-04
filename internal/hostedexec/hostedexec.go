package hostedexec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/requesth"
)

type Client struct {
	HTTPClient  *http.Client
	URL         string
	Credentials credentials.Credentials
}

type RunnerSpec struct {
	Image      string
	EntryPoint string
	Env        []EnvItem
	Files      []FileData
	Artifacts  []string
	Metadata   map[string]string
}

type EnvItem struct {
	Name  string
	Value string
}

type FileData struct {
	Path string
	Data string
}

type Runner struct {
	ID                string
	Status            string
	Image             string
	CreationTime      int64
	TerminationTime   int64
	TerminationReason string
}

type RunnerDetails struct {
	Runner
	Metadata map[string]string
}

type RunnerList struct {
	Content []Runner
}

type EventList struct {
	Content []Event
}

type Event struct {
	CreationTime int64
	Namespace    string
	Key          string
	Summary      string
	Metadata     map[string]string
}

type Status struct {
	Phase    string
	ExitCode int
	Logs     string
}

type Service interface {
	TriggerRun(context.Context, RunnerSpec) (Runner, error)
	// GetAllRuns(ctx context.Context, limit int, offset int) RunnerList
	GetRun(ctx context.Context, id string) (RunnerDetails, error)
	StopRun(ctx context.Context, id string) error
	// GetEvents(ctx context.Context, id string) EventList
	// GetStatus(ctx context.Context, id string) Status
}

func New(url string, creds credentials.Credentials, timeout time.Duration) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: timeout},
		URL:        url,
		Credentials: creds,
	}
}
func (c *Client) TriggerRun(ctx context.Context, spec RunnerSpec) (Runner, error) {
	var runner Runner
	url := fmt.Sprintf("%s/hosted/image/runners", c.URL)
	fmt.Println(url)

	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return runner, err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return runner, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return runner, err
	}
	err = json.Unmarshal(body, &runner)

	return runner, nil
}

func (c *Client) GetRun(ctx context.Context, id string) (RunnerDetails, error) {
	var r RunnerDetails
	url := fmt.Sprintf("%s/hosted/image/runners/%s", c.URL, id)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return r, err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
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
	url := fmt.Sprintf("%s/hosted/image/runners/%s", c.URL, runID)

	req, err := requesth.NewWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
