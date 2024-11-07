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
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/multipartext"
	"github.com/saucelabs/saucectl/internal/storage"
	"golang.org/x/time/rate"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/msg"
)

// Query rate is queryRequestRate per second.
var queryRequestRate = 1
var rateLimitTokenBucket = 10

// APITester describes an interface to the api-testing rest endpoints.
type APITester struct {
	HTTPClient         *retryablehttp.Client
	URL                string
	Username           string
	AccessKey          string
	RequestRateLimiter *rate.Limiter
}

// PublishedTest describes a published test.
type PublishedTest struct {
	Published apitest.Test
}

// VaultErrResponse describes the response when a malformed Vault is unable to be parsed
type VaultErrResponse struct {
	Message struct {
		Errors []vaultErr `json:"errors,omitempty"`
	} `json:"message,omitempty"`
	Status string `json:"status,omitempty"`
}

// DriveErrResponse describes the response when drive API returns an error.
type DriveErrResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type vaultErr struct {
	Field         string                  `json:"field,omitempty"`
	Message       string                  `json:"message,omitempty"`
	Object        string                  `json:"object,omitempty"`
	RejectedValue []apitest.VaultVariable `json:"rejected-value,omitempty"`
}

type vaultFileDeletion struct {
	FileNames []string `json:"fileNames"`
}

// NewAPITester a new instance of APITester.
func NewAPITester(url string, username string, accessKey string, timeout time.Duration) APITester {
	return APITester{
		HTTPClient:         NewRetryableClient(timeout),
		URL:                url,
		Username:           username,
		AccessKey:          accessKey,
		RequestRateLimiter: rate.NewLimiter(rate.Every(time.Duration(1/queryRequestRate)*time.Second), rateLimitTokenBucket),
	}
}

// GetProject returns Project metadata for a given hookID.
func (c *APITester) GetProject(ctx context.Context, hookID string) (apitest.ProjectMeta, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s", c.URL, hookID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apitest.ProjectMeta{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.ProjectMeta{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.ProjectMeta{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.ProjectMeta{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var project apitest.ProjectMeta
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return project, err
	}
	return project, nil
}

func (c *APITester) GetEventResult(ctx context.Context, hookID string, eventID string) (apitest.TestResult, error) {
	if err := c.RequestRateLimiter.Wait(ctx); err != nil {
		return apitest.TestResult{}, err
	}

	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/insights/events/%s", c.URL, hookID, eventID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apitest.TestResult{}, err
	}
	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.TestResult{}, err
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.TestResult{}, errors.New(msg.InternalServerError)
	}
	// 404 needs to be treated differently to ensure calling parent is aware of the specific error.
	// API replies 404 until the event is fully processed.
	if resp.StatusCode == http.StatusNotFound {
		return apitest.TestResult{}, apitest.ErrEventNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.TestResult{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}
	var testResult apitest.TestResult
	if err := json.NewDecoder(resp.Body).Decode(&testResult); err != nil {
		return testResult, err
	}
	return testResult, nil
}

func (c *APITester) GetTest(ctx context.Context, hookID string, testID string) (apitest.Test, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/tests/%s", c.URL, hookID, testID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apitest.Test{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.Test{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.Test{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.Test{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var test PublishedTest
	if err := json.NewDecoder(resp.Body).Decode(&test); err != nil {
		return test.Published, err
	}
	return test.Published, nil
}

func (c *APITester) composeURL(path string, buildID string, format string, tunnel config.Tunnel, taskID string) string {
	// NOTE: API url is not user provided so skip error check
	url, _ := url.Parse(c.URL)
	url.Path = path

	query := url.Query()
	if buildID != "" {
		query.Set("buildId", buildID)
	}
	if format != "" {
		query.Set("format", format)
	}

	if tunnel.Name != "" {
		var t string
		if tunnel.Owner != "" {
			t = fmt.Sprintf("%s:%s", tunnel.Owner, tunnel.Name)
		} else {
			t = fmt.Sprintf("%s:%s", c.Username, tunnel.Name)
		}

		query.Set("tunnelId", t)
	}

	if taskID != "" {
		query.Set("taskId", taskID)
	}

	url.RawQuery = query.Encode()

	return url.String()
}

// GetProjects returns the list of Project available.
func (c *APITester) GetProjects(ctx context.Context) ([]apitest.ProjectMeta, error) {
	url := fmt.Sprintf("%s/api-testing/api/project", c.URL)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []apitest.ProjectMeta{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []apitest.ProjectMeta{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return []apitest.ProjectMeta{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return []apitest.ProjectMeta{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%s'", resp.StatusCode, body)
	}

	var projects []apitest.ProjectMeta
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return projects, err
	}
	return projects, nil
}

// GetHooks returns the list of hooks available.
func (c *APITester) GetHooks(ctx context.Context, projectID string) ([]apitest.Hook, error) {
	url := fmt.Sprintf("%s/api-testing/api/project/%s/hook", c.URL, projectID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []apitest.Hook{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []apitest.Hook{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return []apitest.Hook{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return []apitest.Hook{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%s'", resp.StatusCode, body)
	}

	var hooks []apitest.Hook
	if err := json.NewDecoder(resp.Body).Decode(&hooks); err != nil {
		return hooks, err
	}
	return hooks, nil
}

// RunAllAsync runs all the tests for the project described by hookID and returns without waiting for their results.
func (c *APITester) RunAllAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_run-all", hookID), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	return resp, nil
}

// RunEphemeralAsync runs the tests for the project described by hookID and returns without waiting for their results.
func (c *APITester) RunEphemeralAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_exec", hookID), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	return resp, nil
}

// RunTestAsync runs a single test described by testID for the project described by hookID and returns without waiting for results.
func (c *APITester) RunTestAsync(ctx context.Context, hookID string, testID string, buildID string, tunnel config.Tunnel, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/%s/_run", hookID, testID), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	return resp, nil
}

// RunTagAsync runs all the tests for a testTag for a project described by hookID and returns without waiting for results.
func (c *APITester) RunTagAsync(ctx context.Context, hookID string, testTag string, buildID string, tunnel config.Tunnel, test apitest.TestRequest) (apitest.AsyncResponse, error) {
	url := c.composeURL(fmt.Sprintf("/api-testing/rest/v4/%s/tests/_tag/%s/_run", hookID, testTag), buildID, "", tunnel, "")

	payload, err := json.Marshal(test)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	payloadReader := bytes.NewReader(payload)

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPost, url, payloadReader)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.doAsyncRun(c.HTTPClient, req)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	return resp, nil
}

func (c *APITester) doAsyncRun(client *retryablehttp.Client, request *retryablehttp.Request) (apitest.AsyncResponse, error) {
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return apitest.AsyncResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.AsyncResponse{}, errors.New(msg.InternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.AsyncResponse{}, fmt.Errorf("test execution failed; unexpected response code:'%d', msg:'%v'", resp.StatusCode, string(body))
	}

	var asyncResponse apitest.AsyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&asyncResponse); err != nil {
		return apitest.AsyncResponse{}, err
	}

	return asyncResponse, nil
}

// GetVault returns the vault for the project identified by hookID
func (c *APITester) GetVault(ctx context.Context, hookID string) (apitest.Vault, error) {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/vault", c.URL, hookID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apitest.Vault{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.Vault{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.Vault{}, ErrServerError
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apitest.Vault{}, fmt.Errorf("request failed; unexpected response code:'%d', msg:'%s'", resp.StatusCode, body)
	}

	var vaultResponse apitest.Vault
	if err := json.NewDecoder(resp.Body).Decode(&vaultResponse); err != nil {
		return apitest.Vault{}, err
	}

	return vaultResponse, nil
}

func (c *APITester) PutVault(ctx context.Context, hookID string, vault apitest.Vault) error {
	url := fmt.Sprintf("%s/api-testing/rest/v4/%s/vault", c.URL, hookID)

	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(vault)
	if err != nil {
		return err
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPut, url, &b)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return ErrServerError
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp VaultErrResponse
		if err = json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("request failed; unexpected response code:'%d'; body: %q", resp.StatusCode, body)
		}

		return fmt.Errorf("request failed; unexpected response code: '%d'; err: '%v'", resp.StatusCode, errResp)
	}

	return nil
}

// ListVaultFiles returns the list of files in the vault for the project identified by projectID
func (c *APITester) ListVaultFiles(ctx context.Context, projectID string) ([]apitest.VaultFile, error) {
	filesURL := fmt.Sprintf("%s/api-testing/api/project/%s/drive/files", c.URL, projectID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, filesURL, nil)
	if err != nil {
		return []apitest.VaultFile{}, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []apitest.VaultFile{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return []apitest.VaultFile{}, ErrServerError
	}

	if resp.StatusCode != http.StatusOK {
		return []apitest.VaultFile{}, createError(resp.StatusCode, resp.Body)
	}

	var vaultResponse []apitest.VaultFile
	if err := json.NewDecoder(resp.Body).Decode(&vaultResponse); err != nil {
		return []apitest.VaultFile{}, err
	}

	return vaultResponse, nil
}

// GetVaultFileContent returns the content of a file in the vault for the project identified by projectID
func (c *APITester) GetVaultFileContent(ctx context.Context, projectID string, fileID string) (io.ReadCloser, error) {
	filesURL := fmt.Sprintf("%s/api-testing/api/project/%s/drive/files/%s", c.URL, projectID, fileID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodGet, filesURL, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, ErrServerError
	}

	if resp.StatusCode != http.StatusOK {
		return nil, createError(resp.StatusCode, resp.Body)
	}
	return resp.Body, nil
}

// PutVaultFile stores the content of a file in the vault for the project identified by projectID
func (c *APITester) PutVaultFile(ctx context.Context, projectID string, fileName string, fileBody io.ReadCloser) (apitest.VaultFile, error) {
	multipartReader, contentType, err := multipartext.NewMultipartReader("file", storage.FileInfo{Name: fileName}, fileBody)
	if err != nil {
		return apitest.VaultFile{}, nil
	}

	filesURL := fmt.Sprintf("%s/api-testing/api/project/%s/drive/files", c.URL, projectID)
	req, err := NewRetryableRequestWithContext(ctx, http.MethodPost, filesURL, multipartReader)
	if err != nil {
		return apitest.VaultFile{}, err
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return apitest.VaultFile{}, err
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return apitest.VaultFile{}, ErrServerError
	}

	if resp.StatusCode != http.StatusOK {
		return apitest.VaultFile{}, createError(resp.StatusCode, resp.Body)
	}

	var vaultResponse apitest.VaultFile
	if err := json.NewDecoder(resp.Body).Decode(&vaultResponse); err != nil {
		return apitest.VaultFile{}, err
	}

	return vaultResponse, nil
}

// DeleteVaultFile delete the files in the vault for the project identified by projectID
func (c *APITester) DeleteVaultFile(ctx context.Context, projectID string, fileNames []string) error {
	filesURL := fmt.Sprintf("%s/api-testing/api/project/%s/drive/files/_delete", c.URL, projectID)

	payload, err := json.Marshal(vaultFileDeletion{
		FileNames: fileNames,
	})
	if err != nil {
		return err
	}

	req, err := NewRetryableRequestWithContext(ctx, http.MethodPost, filesURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.AccessKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return ErrServerError
	}

	if resp.StatusCode != http.StatusOK {
		return createError(resp.StatusCode, resp.Body)
	}
	return nil
}

func createError(statusCode int, body io.Reader) error {
	content, _ := io.ReadAll(body)

	var errorDetails DriveErrResponse
	if err := json.Unmarshal(content, &errorDetails); err != nil || errorDetails.Message == "" {
		return fmt.Errorf("request failed; unexpected response code:'%d', body:'%s'", statusCode, content)
	}
	return fmt.Errorf("request failed; unexpected response code:'%d', msg:'%s'", statusCode, errorDetails.Message)
}
