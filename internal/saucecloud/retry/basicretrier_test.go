package retry

import (
	"context"
	"testing"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/stretchr/testify/assert"
)

func TestBasicRetrier_Retry(t *testing.T) {
	type args struct {
		jobOpts  chan job.StartOptions
		opt      job.StartOptions
		previous job.Job
	}
	tests := []struct {
		name     string
		args     args
		expected job.StartOptions
	}{
		{
			name: "Job is retried as-is",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BasicRetrier{}
			go b.Retry(context.Background(), tt.args.jobOpts, tt.args.opt, tt.args.previous)
			newOpt := <-tt.args.jobOpts
			assert.Equal(t, tt.expected, newOpt)
		})
	}
}
