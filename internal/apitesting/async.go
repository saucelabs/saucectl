package apitesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/requesth"
)

type AsyncResponse struct {
	ContextIDs []string
	EventIDs   []string
	TaskID     string
	TestIDs    []string
}

func (c *Client) RunAllAsync(ctx context.Context, hookId string, buildId string) ([]TestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/_run-all", c.URL, hookId)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := doAsyncRun(c.HTTPClient, req)

	apifProject, err := c.GetProject(ctx, hookId)

	if err != nil {
		apifProject = Project{
			ID: "",
			Name: "",
		}
	}

	var testResults []TestResult

	for _, e := range resp.EventIDs {
		testResults = append(testResults, TestResult{
			EventID: e,
			Project: apifProject,
			Async: true,
		})
	}
	return testResults, nil
}

func (c *Client) RunTestAsync(ctx context.Context, hookId string, testId string, buildId string) ([]TestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/%s/_run", c.URL, hookId, testId)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := doAsyncRun(c.HTTPClient, req)

	apifProject, err := c.GetProject(ctx, hookId)

	if err != nil {
		apifProject = Project{
			ID: "",
			Name: "",
		}
	}

	var testResults []TestResult

	for _, e := range resp.EventIDs {
		testResults = append(testResults, TestResult{
			EventID: e,
			Project: apifProject,
			Async: true,
		})
	}
	return testResults, nil
}

func (c *Client) RunTagAsync(ctx context.Context, hookId string, testTag string, buildId string) ([]TestResult, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/_tag/%s/_run", c.URL, hookId, testTag)
	req, err := requesth.NewWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return []TestResult{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := doAsyncRun(c.HTTPClient, req)

	apifProject, err := c.GetProject(ctx, hookId)

	if err != nil {
		apifProject = Project{
			ID: "",
			Name: "",
		}
	}

	var testResults []TestResult

	for _, e := range resp.EventIDs {
		testResults = append(testResults, TestResult{
			EventID: e,
			Project: apifProject,
			Async: true,
		})
	}
	return testResults, nil
}

func doAsyncRun(client *http.Client, request *http.Request) (AsyncResponse, error) {
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return AsyncResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return AsyncResponse{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return AsyncResponse{}, fmt.Errorf("Test execution failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	asyncResponse := AsyncResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&asyncResponse); err != nil {
		return AsyncResponse{}, err
	}

	return asyncResponse, nil
}
