package saucecloud

import (
	"context"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/storage"
	"testing"
)

func TestCloudRunner_logSuiteConsole(t *testing.T) {
	type fields struct {
		ProjectUploader storage.ProjectUploader
		JobStarter      job.Starter
		JobReader       job.Reader
		CCYReader       concurrency.Reader
		Region          region.Region
		ShowConsoleLog  bool
	}
	type args struct {
		res result
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "simple test",
			fields: fields{JobReader: &mocks.FakeJobReader{
				GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
					return []byte("dummy-content"), nil
				},
			}},
			args: args{
				res: result{
					job: job.Job{
						ID: "fake-job-id",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CloudRunner{
				JobReader: tt.fields.JobReader,
			}
			r.logSuiteConsole(tt.args.res)
		})
	}
}
