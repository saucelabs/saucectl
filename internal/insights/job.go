package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/requesth"
)

// ListJobResp represents list job response structure
type ListJobResp struct {
	Jobs  []JobResp `json:"jobs"`
	Total int       `json:"total"`
}

// JobResp represents job response inside of ListJobResp
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

const AutomaticRunMode = "automatic"

// ListJobs returns job list
func (c *Client) ListJobs(ctx context.Context, userID, jobSource string, queryOpts job.QueryOption) (job.List, error) {
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
		return jobList, fmt.Errorf("status: %s", resp.Status)
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

func (c *Client) ReadJob(ctx context.Context, jobID string) (job.Job, error) {
	vdcJob, err := c.ReadVDCJob(ctx, jobID)
	if err != nil {
		rdcJob, err := c.ReadRDCJob(ctx, jobID)
		if err != nil {
			apiJob, err := c.ReadAPIJob(ctx, jobID)
			if err != nil {
				return job.Job{}, fmt.Errorf("failed to get job: %w", err)
			}
			return apiJob, nil
		}
		return rdcJob, nil
	}
	return vdcJob, nil
}

func (c *Client) ReadVDCJob(ctx context.Context, jobID string) (job.Job, error) {
	url := fmt.Sprintf("%s/v2/archives/vdc/jobs/%s", c.URL, jobID)
	return c.doRequest(ctx, url, jobID)
}

func (c *Client) ReadRDCJob(ctx context.Context, jobID string) (job.Job, error) {
	url := fmt.Sprintf("%s/v2/archives/rdc/jobs/%s", c.URL, jobID)
	return c.doRequest(ctx, url, jobID)
}

func (c *Client) ReadAPIJob(ctx context.Context, jobID string) (job.Job, error) {
	url := fmt.Sprintf("%s/v2/archives/api/jobs/%s", c.URL, jobID)
	return c.doRequest(ctx, url, jobID)
}

func (c *Client) doRequest(ctx context.Context, url, jobID string) (job.Job, error) {
	var j job.Job

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
		return j, fmt.Errorf("status: %s", resp.Status)
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