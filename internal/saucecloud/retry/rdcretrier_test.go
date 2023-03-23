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

func TestRDCRetrier_Retry(t *testing.T) {
	type args struct {
		jobOpts  chan job.StartOptions
		opt      job.StartOptions
		previous job.Job
	}
	type init struct {
		Kind   string
		Reader job.Reader
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
					SmartRetry:  false,
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
				Kind: "",
				Reader: &mocks.FakeJobReader{
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
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry:  true,
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
				SmartRetry: true,
			},
		},
		{
			name: "Job retrying only failed suites if RDC + SmartRetry + Keeps espresso fields",
			init: init{
				Kind: "espresso",
				Reader: &mocks.FakeJobReader{
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
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry:  true,
					TestOptions: map[string]interface{}{
						"class":      []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
						"numShards":  2,
						"shardIndex": 1,
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
					"class":      []string{"Demo.Class1"},
					"numShards":  2,
					"shardIndex": 1,
				},
				SmartRetry: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &RDCRetrier{
				Kind:      tt.init.Kind,
				RDCReader: tt.init.Reader,
			}
			go b.Retry(tt.args.jobOpts, tt.args.opt, tt.args.previous)
			newOpt := <-tt.args.jobOpts
			assert.Equal(t, tt.expected, newOpt)
		})
	}
}
