package saucecloud

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/stretchr/testify/assert"
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
				StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
					return "fake-id", false, nil
				},
			},
			JobReader: &mocks.FakeJobReader{
				PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
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

func TestRunJobTimeout(t *testing.T) {
	r := CloudRunner{
		JobStarter: &mocks.FakeJobStarter{
			StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
				return "1", false, nil
			},
		},
		JobReader: &mocks.FakeJobReader{
			PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
				return job.Job{ID: id, TimedOut: true}, nil
			},
		},
		JobStopper: &mocks.FakeJobStopper{
			StopJobFn: func(ctx context.Context, jobID string) (job.Job, error) {
				return job.Job{ID: jobID}, nil
			},
		},
		JobWriter: &mocks.FakeJobWriter{UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
			return nil
		}},
	}

	opts := make(chan job.StartOptions)
	results := make(chan result)

	go r.runJobs(opts, results)
	opts <- job.StartOptions{
		DisplayName: "dummy",
		Timeout:     1,
	}
	close(opts)
	res := <-results
	assert.Error(t, res.err, "suite 'dummy' has reached timeout")
	assert.True(t, res.job.TimedOut)
}

func TestRunJobRetries(t *testing.T) {
	type testCase struct {
		retries      int
		wantAttempts int
	}

	tests := []testCase{
		{
			retries:      0,
			wantAttempts: 1,
		},
		{
			retries:      4,
			wantAttempts: 5,
		},
	}
	for _, tt := range tests {
		r := CloudRunner{
			JobStarter: &mocks.FakeJobStarter{
				StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
					return "1", false, nil
				},
			},
			JobReader: &mocks.FakeJobReader{
				PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
					return job.Job{ID: id, Passed: false}, nil
				},
			},
			JobStopper: &mocks.FakeJobStopper{
				StopJobFn: func(ctx context.Context, jobID string) (job.Job, error) {
					return job.Job{ID: jobID}, nil
				},
			},
			JobWriter: &mocks.FakeJobWriter{UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
				return nil
			}},
		}

		opts := make(chan job.StartOptions, tt.retries+1)
		results := make(chan result)

		go r.runJobs(opts, results)
		opts <- job.StartOptions{
			DisplayName: "retry job",
			Retries:     tt.retries,
		}
		res := <-results
		close(opts)
		close(results)
		assert.Equal(t, res.attempts, tt.wantAttempts)
	}
}

func TestRunJobTimeoutRDC(t *testing.T) {
	r := CloudRunner{
		JobStarter: &mocks.FakeJobStarter{
			StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
				return "1", true, nil
			},
		},
		RDCJobReader: &mocks.FakeJobReader{
			PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
				return job.Job{ID: id, TimedOut: true}, nil
			},
		},
	}

	opts := make(chan job.StartOptions)
	results := make(chan result)

	go r.runJobs(opts, results)
	opts <- job.StartOptions{
		DisplayName: "dummy",
		Timeout:     1,
	}
	close(opts)
	res := <-results
	assert.Error(t, res.err, "suite 'dummy' has reached timeout")
	assert.True(t, res.job.TimedOut)
}
