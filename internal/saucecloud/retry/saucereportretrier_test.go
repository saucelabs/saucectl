package retry

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/stretchr/testify/assert"
)

type StubProjectUploader struct {
}

func (f *StubProjectUploader) UploadStream(filename, description string, reader io.Reader) (storage.Item, error) {
	return storage.Item{
		ID:   "fakeid",
		Name: "fake name",
	}, nil
}

func (f *StubProjectUploader) Download(id string) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

func (f *StubProjectUploader) DownloadURL(url string) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

func (f *StubProjectUploader) List(opts storage.ListOptions) (storage.List, error) {
	return storage.List{}, nil
}

type StubVDCJobReader struct {
	SauceReport saucereport.SauceReport
}

func (f *StubVDCJobReader) ReadJob(ctx context.Context, id string, realDevice bool) (job.Job, error) {
	return job.Job{}, nil
}

func (f *StubVDCJobReader) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
	return job.Job{}, nil
}

func (f *StubVDCJobReader) GetJobAssetFileNames(ctx context.Context, jobID string, realDevice bool) ([]string, error) {
	return []string{}, nil
}

func (f *StubVDCJobReader) GetJobAssetFileContent(ctx context.Context, jobID, fileName string, realDevice bool) ([]byte, error) {
	body, _ := json.Marshal(f.SauceReport)
	return []byte(body), nil
}

type FakeProject struct {
}

func (p *FakeProject) FilterFailedTests(index int, report saucereport.SauceReport) error {
	return nil
}

func TestSauceReportRetrier_Retry(t *testing.T) {
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
		name     string
		retrier  *SauceReportRetrier
		args     args
		expected job.StartOptions
	}{
		{
			name:    "Job is resent as-it",
			retrier: &SauceReportRetrier{},
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
			name: "Job is set as SmartRetry",
			retrier: &SauceReportRetrier{
				VDCReader:       &StubVDCJobReader{SauceReport: failedReport},
				ProjectUploader: &StubProjectUploader{},
				Project:         &FakeProject{},
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Try failed tests",
					SuiteIndex:  0,
					Framework:   cypress.Kind,
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
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
					FailedOnly: true,
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
		})
	}
}
