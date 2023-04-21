package retry

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/cucumber"
	"github.com/saucelabs/saucectl/internal/cypress"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/stretchr/testify/assert"
)

type FakeProjectUploader struct {
}

func (f *FakeProjectUploader) UploadStream(filename, description string, reader io.Reader) (storage.Item, error) {
	return storage.Item{
		ID:   "fakeid",
		Name: "fake name",
	}, nil
}

func (f *FakeProjectUploader) Download(id string) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

func (f *FakeProjectUploader) DownloadURL(url string) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

func (f *FakeProjectUploader) List(opts storage.ListOptions) (storage.List, error) {
	return storage.List{}, nil
}

type FakeVDCJobReader struct {
	SauceReport saucereport.SauceReport
}

func (f *FakeVDCJobReader) ReadJob(ctx context.Context, id string, realDevice bool) (job.Job, error) {
	return job.Job{}, nil
}

func (f *FakeVDCJobReader) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
	return job.Job{}, nil
}

func (f *FakeVDCJobReader) GetJobAssetFileNames(ctx context.Context, jobID string, realDevice bool) ([]string, error) {
	return []string{}, nil
}

func (f *FakeVDCJobReader) GetJobAssetFileContent(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error) {
	body, _ := json.Marshal(f.SauceReport)
	return []byte(body), nil
}

func TestBasicRetrier_Retry(t *testing.T) {
	failedReport := saucereport.SauceReport{
		Status: saucereport.StatusFailed,
		Suites: []saucereport.Suite{
			{
				Name:   "first suite",
				Status: saucereport.StatusFailed,
				Tests: []saucereport.Test{
					{
						Name:   "passed test",
						Status: saucereport.StatusPassed,
					},
					{
						Name:   "failed test",
						Status: saucereport.StatusFailed,
					},
				},
			},
			{
				Name:   "second suite",
				Status: saucereport.StatusFailed,
				Suites: []saucereport.Suite{
					{
						Name:   "third suite",
						Status: saucereport.StatusFailed,
						Tests: []saucereport.Test{
							{
								Name:   "passed test2",
								Status: saucereport.StatusPassed,
							},
							{
								Name:   "failed test2",
								Status: saucereport.StatusFailed,
							},
						},
					},
				},
			},
		},
	}
	type args struct {
		jobOpts  chan job.StartOptions
		opt      job.StartOptions
		previous job.Job
	}
	tests := []struct {
		name                 string
		retrier              *BasicRetrier
		args                 args
		expected             job.StartOptions
		expCucumberProject   cucumber.Project
		expCypressProject    cypress.Project
		expPlaywrightProject playwright.Project
		expTestCafeProject   testcafe.Project
	}{
		{
			name:    "Job is resent as-it",
			retrier: &BasicRetrier{},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
				},
				previous: job.Job{},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
			},
		},
		{
			name: "Cucumber Job is set as SmartRetry",
			retrier: &BasicRetrier{
				VDCReader:       &FakeVDCJobReader{SauceReport: failedReport},
				ProjectUploader: &FakeProjectUploader{},
				CucumberProject: cucumber.Project{
					Suites: []cucumber.Suite{
						{
							Options: cucumber.Options{},
						},
					},
				},
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Try failed tests",
					SuiteIndex:  0,
					Framework:   cucumber.Kind,
					SmartRetry: job.SmartRetry{
						FailedTestsOnly: true,
					},
				},
				previous: job.Job{},
			},
			expected: job.StartOptions{
				DisplayName: "Try failed tests",
				SuiteIndex:  0,
				Framework:   cucumber.Kind,
				OtherApps:   []string{"storage:fakeid"},
				SmartRetry: job.SmartRetry{
					FailedTestsOnly: true,
				},
			},
			expCucumberProject: cucumber.Project{
				Suites: []cucumber.Suite{
					{
						Options: cucumber.Options{
							Name: "failed test|failed test2",
						},
					},
				},
			},
		},
		{
			name: "Cypress Job is set as SmartRetry",
			retrier: &BasicRetrier{
				VDCReader:       &FakeVDCJobReader{SauceReport: failedReport},
				ProjectUploader: &FakeProjectUploader{},
				CypressProject: &v1.Project{
					Suites: []v1.Suite{
						{
							Config: v1.SuiteConfig{},
						},
					},
				},
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Try failed tests",
					SuiteIndex:  0,
					Framework:   cypress.Kind,
					SmartRetry: job.SmartRetry{
						FailedTestsOnly: true,
					},
				},
				previous: job.Job{},
			},
			expected: job.StartOptions{
				DisplayName: "Try failed tests",
				SuiteIndex:  0,
				Framework:   cypress.Kind,
				OtherApps:   []string{"storage:fakeid"},
				SmartRetry: job.SmartRetry{
					FailedTestsOnly: true,
				},
			},
			expCypressProject: &v1.Project{
				Suites: []v1.Suite{
					{
						Config: v1.SuiteConfig{
							Env: map[string]string{
								"grep": "failed test;failed test2",
							},
						},
					},
				},
			},
		},
		{
			name: "Playwright Job is set as SmartRetry",
			retrier: &BasicRetrier{
				VDCReader:       &FakeVDCJobReader{SauceReport: failedReport},
				ProjectUploader: &FakeProjectUploader{},
				PlaywrightProject: playwright.Project{
					Suites: []playwright.Suite{
						{
							Params: playwright.SuiteConfig{},
						},
					},
				},
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Try failed tests",
					SuiteIndex:  0,
					Framework:   playwright.Kind,
					SmartRetry: job.SmartRetry{
						FailedTestsOnly: true,
					},
				},
				previous: job.Job{},
			},
			expected: job.StartOptions{
				DisplayName: "Try failed tests",
				SuiteIndex:  0,
				Framework:   playwright.Kind,
				OtherApps:   []string{"storage:fakeid"},
				SmartRetry: job.SmartRetry{
					FailedTestsOnly: true,
				},
			},
			expPlaywrightProject: playwright.Project{
				Suites: []playwright.Suite{
					{
						Params: playwright.SuiteConfig{
							Grep: "failed test|failed test2",
						},
					},
				},
			},
		},
		{
			name: "TestCafe Job is set as SmartRetry",
			retrier: &BasicRetrier{
				VDCReader:       &FakeVDCJobReader{SauceReport: failedReport},
				ProjectUploader: &FakeProjectUploader{},
				TestcafeProject: testcafe.Project{
					Suites: []testcafe.Suite{
						{
							Filter: testcafe.Filter{},
						},
					},
				},
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Try failed tests",
					SuiteIndex:  0,
					Framework:   testcafe.Kind,
					SmartRetry: job.SmartRetry{
						FailedTestsOnly: true,
					},
				},
				previous: job.Job{},
			},
			expected: job.StartOptions{
				DisplayName: "Try failed tests",
				SuiteIndex:  0,
				Framework:   testcafe.Kind,
				OtherApps:   []string{"storage:fakeid"},
				SmartRetry: job.SmartRetry{
					FailedTestsOnly: true,
				},
			},
			expTestCafeProject: testcafe.Project{
				Suites: []testcafe.Suite{
					{
						Filter: testcafe.Filter{
							TestGrep: "failed test|failed test2",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.retrier
			go b.Retry(tt.args.jobOpts, tt.args.opt, tt.args.previous)
			newOpt := <-tt.args.jobOpts
			assert.Equal(t, tt.expected, newOpt)
			assert.Equal(t, tt.expCucumberProject, tt.retrier.CucumberProject)
			assert.Equal(t, tt.expCypressProject, tt.retrier.CypressProject)
			assert.Equal(t, tt.expPlaywrightProject, tt.retrier.PlaywrightProject)
			assert.Equal(t, tt.expTestCafeProject, tt.retrier.TestcafeProject)
		})
	}
}
