package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/job"

	"github.com/saucelabs/saucectl/internal/iam"
)

// archivesJobList represents list job response structure
type archivesJobList struct {
	Jobs  []archivesJob `json:"jobs"`
	Total int           `json:"total"`
}

// archivesJob represents job response structure
type archivesJob struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Error       string `json:"error"`
	Framework   string `json:"automation_backend"`
	Device      string `json:"device"`
	BrowserName string `json:"browser_name"`
	OS          string `json:"os"`
	OSVersion   string `json:"os_version"`
	Source      string `json:"source"`
}

// AutomaticRunMode indicates the job is automated
const AutomaticRunMode = "automatic"

type InsightsService struct {
	HTTPClient  *http.Client
	URL         string
	Credentials iam.Credentials
}

func NewInsightsService(url string, creds iam.Credentials, timeout time.Duration) InsightsService {
	return InsightsService{
		HTTPClient: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
		},
		URL:         url,
		Credentials: creds,
	}
}

func (c *InsightsService) GetHistory(ctx context.Context, user iam.User, sortBy string) (insights.JobHistory, error) {
	start := time.Now().AddDate(0, 0, -7).Unix()
	now := time.Now().Unix()

	var jobHistory insights.JobHistory
	url := fmt.Sprintf("%s/insights/v2/test-cases", c.URL)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return jobHistory, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	q := req.URL.Query()
	queries := map[string]string{
		"user_id": user.ID,
		"org_id":  user.Organization.ID,
		"start":   strconv.FormatInt(start, 10),
		"since":   strconv.FormatInt(start, 10),
		"end":     strconv.FormatInt(now, 10),
		"until":   strconv.FormatInt(now, 10),
		"limit":   "200",
		"offset":  "0",
		"sort_by": sortBy,
	}
	for k, v := range queries {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return jobHistory, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return jobHistory, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return jobHistory, json.NewDecoder(resp.Body).Decode(&jobHistory)
}

type testRunsInput struct {
	TestRuns []insights.TestRun `json:"test_runs,omitempty"`
}

type testRunError struct {
	Loc  []interface{} `json:"loc,omitempty"`
	Msg  string        `json:"msg,omitempty"`
	Type string        `json:"type,omitempty"`
}

type testRunErrorResponse struct {
	Detail []testRunError `json:"detail,omitempty"`
}

func concatenateLocation(loc []interface{}) string {
	out := ""
	for idx, item := range loc {
		if idx > 0 {
			out += "."
		}
		if reflect.TypeOf(item).String() == "string" {
			out = fmt.Sprintf("%s%s", out, item)
		}
		if reflect.TypeOf(item).String() == "int" {
			out = fmt.Sprintf("%s%d", out, item)
		}
	}
	return out
}

// PostTestRun publish test-run results to insights API.
func (c *InsightsService) PostTestRun(ctx context.Context, runs []insights.TestRun) error {
	url := fmt.Sprintf("%s/test-runs/v1/", c.URL)

	input := testRunsInput{
		TestRuns: runs,
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	payloadReader := bytes.NewReader(payload)
	req, err := NewRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	// API Replies 204, doc says 200. Supporting both for now.
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var res testRunErrorResponse
		if err = json.Unmarshal(body, &res); err != nil {
			return fmt.Errorf("unable to unmarshal response from API: %s", err)
		}
		return fmt.Errorf("%s: %s", concatenateLocation(res.Detail[0].Loc), res.Detail[0].Type)
	}
	return fmt.Errorf("unexpected status code from API: %d", resp.StatusCode)
}

// ListJobs returns job list
func (c *InsightsService) ListJobs(ctx context.Context, opts insights.ListJobsOptions) ([]job.Job, error) {
	url := fmt.Sprintf("%s/v2/archives/jobs", c.URL)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	q := req.URL.Query()
	queries := map[string]string{
		"ts":       strconv.FormatInt(time.Now().UTC().UnixMilli(), 10),
		"page":     strconv.Itoa(opts.Page),
		"size":     strconv.Itoa(opts.Size),
		"status":   opts.Status,
		"owner_id": opts.UserID,
		"run_mode": AutomaticRunMode,
		"source":   string(opts.Source),
	}
	for k, v := range queries {
		if v != "" {
			q.Add(k, v)
		}
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return c.parseJobs(resp.Body)
}

func (c *InsightsService) ReadJob(ctx context.Context, id string) (job.Job, error) {
	url := fmt.Sprintf("%s/v2/archives/jobs/%s", c.URL, id)

	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return job.Job{}, err
	}

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return job.Job{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return job.Job{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return c.parseJob(resp.Body)
}

// parseJob parses the body into archivesJob and converts it to job.Job.
func (c *InsightsService) parseJob(body io.ReadCloser) (job.Job, error) {
	var j archivesJob

	if err := json.NewDecoder(body).Decode(&j); err != nil {
		return job.Job{}, err
	}

	return c.convertJob(j), nil
}

// parseJob parses the body into archivesJobList and converts it to []job.Job.
func (c *InsightsService) parseJobs(body io.ReadCloser) ([]job.Job, error) {
	var l archivesJobList

	if err := json.NewDecoder(body).Decode(&l); err != nil {
		return nil, err
	}

	jobs := make([]job.Job, len(l.Jobs))
	for i, j := range l.Jobs {
		jobs[i] = c.convertJob(j)
	}

	return jobs, nil
}

// parseJob converts archivesJob to job.Job.
func (c *InsightsService) convertJob(j archivesJob) job.Job {
	return job.Job{
		ID:          j.ID,
		Name:        j.Name,
		Status:      j.Status,
		Error:       j.Error,
		OS:          j.OS,
		OSVersion:   j.OSVersion,
		Framework:   j.Framework,
		DeviceName:  j.Device,
		BrowserName: j.BrowserName,
	}
}
