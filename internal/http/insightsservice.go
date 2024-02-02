package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/job"

	"github.com/saucelabs/saucectl/internal/config"
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

// GetHistory returns job history from insights
func (c *InsightsService) GetHistory(ctx context.Context, user iam.User, launchOrder config.LaunchOrder) (insights.JobHistory, error) {
	vdc, err := c.doGetHistory(ctx, user, launchOrder, "vdc")
	if err != nil {
		return insights.JobHistory{}, err
	}
	rdc, err := c.doGetHistory(ctx, user, launchOrder, "rdc")
	if err != nil {
		return insights.JobHistory{}, err
	}

	jobHistory := mergeJobHistories([]insights.JobHistory{vdc, rdc})
	return jobHistory, nil
}

func mergeJobHistories(histories []insights.JobHistory) insights.JobHistory {
	testCasesMap := map[string]insights.TestCase{}
	for _, history := range histories {
		for _, tc := range history.TestCases {
			addOrReplaceTestCase(testCasesMap, tc)
		}
	}
	var testCases []insights.TestCase
	for _, tc := range testCasesMap {
		testCases = append(testCases, tc)
	}
	sort.Slice(testCases, func(i, j int) bool {
		return testCases[i].FailRate > testCases[j].FailRate
	})
	return insights.JobHistory{
		TestCases: testCases,
	}
}

// addOrReplaceTestCase adds or replaces the insights.TestCase in the map[string]insights.TestCase
// If there is already one with the same name, only the highest fail rate is kept.
func addOrReplaceTestCase(mp map[string]insights.TestCase, tc insights.TestCase) {
	tcRef, present := mp[tc.Name]
	if !present {
		mp[tc.Name] = tc
		return
	}
	if tc.FailRate > tcRef.FailRate {
		mp[tc.Name] = tc
	}
}

func (c *InsightsService) doGetHistory(ctx context.Context, user iam.User, launchOrder config.LaunchOrder, source string) (insights.JobHistory, error) {
	start := time.Now().AddDate(0, 0, -7).Unix()
	now := time.Now().Unix()

	var jobHistory insights.JobHistory
	url := fmt.Sprintf("%s/v2/insights/%s/test-cases", c.URL, source)
	req, err := NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		"source":   opts.Source,
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
		ID:              j.ID,
		Name:            j.Name,
		Status:          j.Status,
		Error:           j.Error,
		PlatformName:    j.OS,
		PlatformVersion: j.OSVersion,
		Framework:       j.Framework,
		DeviceName:      j.Device,
		BrowserName:     j.BrowserName,
	}
}
