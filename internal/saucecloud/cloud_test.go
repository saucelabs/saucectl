package saucecloud

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestCloudRunner_logSuiteConsole(t *testing.T) {
	type fields struct {
		ProjectUploader storage.ProjectUploader
		JobStarter      job.Starter
		JobReader       job.Reader
		JobWriter       job.Writer
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
			fields: fields{
				JobReader: &mocks.FakeJobReader{
					GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
						return []byte("dummy-content"), nil
					},
				},
				JobWriter: &mocks.FakeJobWriter{
					UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
						return nil
					},
				},
			},
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
				JobWriter: tt.fields.JobWriter,
			}
			r.logSuiteConsole(tt.args.res)
		})
	}
}

func TestSignalDetection(t *testing.T) {
	r := CloudRunner{JobStopper: &mocks.FakeJobStopper{}}
	assert.False(t, r.interrupted)
	c := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(c)

	c <- syscall.SIGINT

	time.Sleep(1 * time.Second)
	assert.True(t, r.interrupted)
}

func TestSignalDetectionExit(t *testing.T) {
	if os.Getenv("FORCE_EXIT_TEST") == "1" {
		r := CloudRunner{JobStopper: &mocks.FakeJobStopper{}}
		assert.False(t, r.interrupted)
		c := r.registerSkipSuitesOnSignal()
		defer unregisterSignalCapture(c)

		c <- syscall.SIGINT
		c <- syscall.SIGINT
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestSignalDetectionExit")
	cmd.Env = append(os.Environ(), "FORCE_EXIT_TEST=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestSkippedRunJobs(t *testing.T) {
	type testCase struct {
		interrupted bool
		wantErr     bool
		wantSkipped bool
		wantJobID   bool
	}
	tests := []testCase{
		{
			interrupted: true,
			wantSkipped: true,
			wantErr:     false,
			wantJobID:   false,
		},
	}
	for _, tt := range tests {
		r := CloudRunner{
			JobStarter: &mocks.FakeJobStarter{
				StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, err error) {
					return "fake-id", nil
				},
			},
			JobReader: &mocks.FakeJobReader{
				PollJobFn: func(ctx context.Context, id string, interval time.Duration) (job.Job, error) {
					return job.Job{
						ID:     "fake-id",
						Passed: true,
						Error:  "",
						Status: job.StateComplete,
					}, nil
				},
			},
			JobWriter: &mocks.FakeJobWriter{
				UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
					return nil
				},
			},
		}
		r.interrupted = tt.interrupted

		j, skipped, err := r.runJob(job.StartOptions{})
		assert.Equal(t, tt.wantSkipped, skipped)
		assert.Equal(t, tt.wantErr, err != nil)
		assert.Equal(t, tt.wantJobID, j.ID != "")
	}
}

func TestRunJobsSkipped(t *testing.T) {
	r := CloudRunner{}
	r.interrupted = true

	opts := make(chan job.StartOptions)
	results := make(chan result)

	go r.runJobs(opts, results)
	opts <- job.StartOptions{}
	close(opts)
	res := <-results
	assert.Nil(t, res.err)
	assert.True(t, res.skipped)
}

func TestDownloadArtifacts(t *testing.T) {
	dir := fs.NewDir(t, "download-artifacts")
	defer dir.Remove()

	r := CloudRunner{
		JobReader: &mocks.FakeJobReader{
			GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
				return []string{"console.log", "dummy-file.log"}, nil
			},
			GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
				if fileName == "console.log" {
					return []byte("console-log-content"), nil
				}
				if fileName == "dummy-file.log" {
					return []byte("dummy-file-log-content"), nil
				}
				return nil, errors.New("invalid-file")
			},
		},
	}
	cfg := config.ArtifactDownload{
		When:      config.WhenAlways,
		Directory: filepath.Join(dir.Path(), "results"),
		Match:     []string{"console.log"},
	}
	j := job.Job{
		ID:     "fake-job-id",
		Status: job.StateComplete,
	}
	expectedFiles := []struct {
		filename string
		content  []byte
	}{
		{filename: "console.log", content: []byte("console-log-content")},
	}
	r.downloadArtifacts(cfg, j, true)
	for _, expectedFile := range expectedFiles {
		content, err := os.ReadFile(filepath.Join(cfg.Directory, j.ID, expectedFile.filename))
		if err != nil {
			t.Errorf("unable to read expected file: %v (%v)", expectedFile.filename, err)
		}
		if !reflect.DeepEqual(content, expectedFile.content) {
			t.Errorf("file content differs: %v", expectedFile.filename)
		}
	}
}
