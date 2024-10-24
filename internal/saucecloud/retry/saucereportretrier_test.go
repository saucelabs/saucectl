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

func (f *StubProjectUploader) UploadStream(_, _ string, _ io.Reader) (storage.Item, error) {
	return storage.Item{
		ID:   "fakeid",
		Name: "fake name",
	}, nil
}

func (f *StubProjectUploader) Download(string) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

func (f *StubProjectUploader) DownloadURL(string) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

func (f *StubProjectUploader) List(storage.ListOptions) (storage.List, error) {
	return storage.List{}, nil
}

func (f *StubProjectUploader) Delete(string) error {
	return nil
}

type StubVDCJobReader struct {
	SauceReport saucereport.SauceReport
}

func (f *StubVDCJobReader) ReadJob(context.Context, string, bool) (job.Job, error) {
	return job.Job{}, nil
}

func (f *StubVDCJobReader) PollJob(_ context.Context, _ string, _, _ time.Duration, _ bool) (job.Job, error) {
	return job.Job{}, nil
}

func (f *StubVDCJobReader) GetJobAssetFileNames(context.Context, string, bool) ([]string, error) {
	return []string{}, nil
}

func (f *StubVDCJobReader) GetJobAssetFileContent(_ context.Context, _, _ string, _ bool) ([]byte, error) {
	body, _ := json.Marshal(f.SauceReport)
	return []byte(body), nil
}

type StubProject struct {
}

func (p *StubProject) FilterFailedTests(string, saucereport.SauceReport) error {
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
			name:    "Job is retried as-is",
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
			name: "Job is retried via SmartRetry",
			retrier: &SauceReportRetrier{
				VDCReader:       &StubVDCJobReader{SauceReport: failedReport},
				ProjectUploader: &StubProjectUploader{},
				Project:         &StubProject{},
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Try failed tests",
					Framework:   cypress.Kind,
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
				},
				previous: job.Job{},
			},
			expected: job.StartOptions{
				DisplayName: "Try failed tests",
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
