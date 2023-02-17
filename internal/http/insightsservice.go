package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/saucelabs/saucectl/internal/cmd/jobs/job"
	"github.com/saucelabs/saucectl/internal/insights"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/requesth"
)

const (
	RDCSource = "rdc"
	VDCSource = "vdc"
	APISource = "api"
)

// ListJobResp represents list job response structure
type ListJobResp struct {
	Jobs  []JobResp `json:"jobs"`
	Total int       `json:"total"`
}

// JobResp represents job response structure
type JobResp struct {
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
	Credentials credentials.Credentials
}

var LaunchOptions = map[config.LaunchOrder]string{
	config.LaunchOrderFailRate: "fail_rate",
}

func NewInsightsService(url string, creds credentials.Credentials, timeout time.Duration) InsightsService {
	return InsightsService{
		HTTPClient:  &http.Client{Timeout: timeout},
		URL:         url,
		Credentials: creds,
	}
}

// GetHistory returns job history from insights
func (c *InsightsService) GetHistory(ctx context.Context, user iam.User, launchOrder config.LaunchOrder) (insights.JobHistory, error) {
	start := time.Now().AddDate(0, 0, -7).Unix()
	now := time.Now().Unix()

	var jobHistory insights.JobHistory
	url := fmt.Sprintf("%s/v2/insights/vdc/test-cases", c.URL)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return jobHistory, err
	}

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
		"sort_by": string(launchOrder),
	}
	for k, v := range queries {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return jobHistory, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return jobHistory, err
	}

	err = json.Unmarshal(body, &jobHistory)
	if err != nil {
		return jobHistory, err
	}
	return jobHistory, nil
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
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, payloadReader)
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
func (c *InsightsService) ListJobs(ctx context.Context, userID, jobSource string, queryOpts job.QueryOption) (job.List, error) {
	var jobList job.List

	url := fmt.Sprintf("%s/v2/archives/jobs", c.URL)
	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return jobList, err
	}
	q := req.URL.Query()
	queries := map[string]string{
		"ts":       strconv.FormatInt(time.Now().UTC().UnixMilli(), 10),
		"page":     strconv.Itoa(queryOpts.Page),
		"size":     strconv.Itoa(queryOpts.Size),
		"status":   queryOpts.Status,
		"owner_id": userID,
		"run_mode": AutomaticRunMode,
		"source":   jobSource,
	}
	for k, v := range queries {
		if v != "" {
			q.Add(k, v)
		}
	}
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return jobList, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return jobList, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return jobList, err
	}
	var listResp ListJobResp
	err = json.Unmarshal(body, &listResp)
	if err != nil {
		return jobList, err
	}
	for _, j := range listResp.Jobs {
		jobList.Jobs = append(jobList.Jobs, buildJob(j))
	}
	jobList.Total = listResp.Total
	jobList.Page = queryOpts.Page
	jobList.Size = queryOpts.Size
	return jobList, nil
}

func buildJob(j JobResp) job.Job {
	var platform string
	if j.OS != "" && j.OSVersion != "" {
		platform = fmt.Sprintf("%s %s", j.OS, j.OSVersion)
	}
	return job.Job{
		ID:          j.ID,
		Name:        j.Name,
		Status:      j.Status,
		Error:       j.Error,
		Platform:    platform,
		Framework:   j.Framework,
		Device:      j.Device,
		BrowserName: j.BrowserName,
		Source:      j.Source,
	}
}

func (c *InsightsService) ReadJob(ctx context.Context, jobID string) (job.Job, error) {
	var source = VDCSource

	switch source {
	case VDCSource:
		vdcJob, err := c.readJob(ctx, jobID, VDCSource)
		if err == nil {
			return vdcJob, nil
		}
		fallthrough
	case RDCSource:
		rdcJob, err := c.readJob(ctx, jobID, RDCSource)
		if err == nil {
			return rdcJob, nil
		}
		fallthrough
	case APISource:
		apiJob, err := c.readJob(ctx, jobID, APISource)
		if err != nil {
			return job.Job{}, fmt.Errorf("failed to get job: %w", err)
		}
		return apiJob, nil
	}
	return job.Job{}, nil
}

func (c *InsightsService) readJob(ctx context.Context, jobID string, jobSource string) (job.Job, error) {
	var j job.Job

	url := fmt.Sprintf("%s/v2/archives/%s/jobs/%s", c.URL, jobSource, jobID)

	req, err := requesth.NewWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return j, err
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return j, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return j, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return j, err
	}
	var jobResp JobResp
	err = json.Unmarshal(body, &jobResp)
	if err != nil {
		return j, err
	}

	return buildJob(jobResp), nil
}
