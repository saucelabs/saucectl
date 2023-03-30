package retry

import (
	"context"
	"errors"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAppsRetrier_Retry(t *testing.T) {
	type args struct {
		jobOpts  chan job.StartOptions
		opt      job.StartOptions
		previous job.Job
	}
	type init struct {
		RDCReader job.Reader
		VDCReader job.Reader
		RetryRDC  bool
		RetryVDC  bool
	}
	tests := []struct {
		name     string
		init     init
		args     args
		expected job.StartOptions
	}{
		{
			name: "Job is resent as-it if no RDC",
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					TestOptions: map[string]interface{}{
						"class": []string{"present"},
					},
				},
				previous: job.Job{
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"present"},
				},
			},
		},
		{
			name: "Job is untouched if there is no SmartRetries and is RDC",
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedClassesOnly: false,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"present"},
					},
				},
				previous: job.Job{
					IsRDC: true,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"present"},
				},
			},
		},
		{
			name: "Job retrying only failed suites if RDC + SmartRetry",
			init: init{
				RDCReader: &mocks.FakeJobReader{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.JunitFileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Demo.Class1\">\n        <failure>ERROR</failure>\n    </testcase>\n    <testcase classname=\"Demo.Class1\"/>\n    <testcase classname=\"Demo.Class2\"/>\n    <testcase classname=\"Demo.Class3\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryRDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedClassesOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: true,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1"},
				},
				SmartRetry: job.SmartRetry{
					FailedClassesOnly: true,
				},
			},
		},
		{
			name: "Job not retrying if RDC and config is VDC + SmartRetry",
			init: init{
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedClassesOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: true,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
				},
				SmartRetry: job.SmartRetry{
					FailedClassesOnly: true,
				},
			},
		},
		{
			name: "Job is retrying when VDC + SmartRetry",
			init: init{
				VDCReader: &mocks.FakeJobReader{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.JunitFileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Demo.Class1\">\n        <failure>ERROR</failure>\n    </testcase>\n    <testcase classname=\"Demo.Class1\"/>\n    <testcase classname=\"Demo.Class2\"/>\n    <testcase classname=\"Demo.Class3\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedClassesOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1"},
				},
				SmartRetry: job.SmartRetry{
					FailedClassesOnly: true,
				},
			},
		},
		{
			name: "Base Retry if junit is malformed",
			init: init{
				VDCReader: &mocks.FakeJobReader{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.JunitFileName {
							return []byte("malformed"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedClassesOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
				},
				SmartRetry: job.SmartRetry{
					FailedClassesOnly: true,
				},
			},
		},
		{
			name: "Base Retry if getting junit.xml is failing",
			init: init{
				VDCReader: &mocks.FakeJobReader{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.JunitFileName {
							return []byte("malformed"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedClassesOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-buggy-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
				},
				SmartRetry: job.SmartRetry{
					FailedClassesOnly: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &JunitRetrier{
				RDCReader: tt.init.RDCReader,
				VDCReader: tt.init.VDCReader,
				RetryRDC:  tt.init.RetryRDC,
				RetryVDC:  tt.init.RetryVDC,
			}
			go b.Retry(tt.args.jobOpts, tt.args.opt, tt.args.previous)
			newOpt := <-tt.args.jobOpts
			assert.Equal(t, tt.expected, newOpt)
		})
	}
}
