package saucecloud

import (
	"context"
	"errors"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"gotest.tools/v3/fs"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestSignalDetection(t *testing.T) {
	r := CloudRunner{JobService: JobService{VDCStopper: &mocks.FakeJobStopper{}}}
	assert.False(t, r.interrupted)
	c := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(c)

	c <- syscall.SIGINT

	time.Sleep(1 * time.Second)
	assert.True(t, r.interrupted)
}

func TestSignalDetectionExit(t *testing.T) {
	if os.Getenv("FORCE_EXIT_TEST") == "1" {
		r := CloudRunner{JobService: JobService{VDCStopper: &mocks.FakeJobStopper{}}}
		assert.False(t, r.interrupted)
		c := r.registerSkipSuitesOnSignal()
		defer unregisterSignalCapture(c)

		c <- syscall.SIGINT
		time.Sleep(1 * time.Second)
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
	sut := CloudRunner{
		JobService: JobService{
			VDCStarter: &mocks.FakeJobStarter{
				StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
					return "fake-id", false, nil
				},
			},
			VDCStopper: &mocks.FakeJobStopper{
				StopJobFn: func(ctx context.Context, id string) (job.Job, error) {
					return job.Job{
						ID: "fake-id",
					}, nil
				},
			},
			VDCReader: &mocks.FakeJobReader{
				PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
					return job.Job{
						ID:     "fake-id",
						Passed: true,
						Error:  "",
						Status: job.StateComplete,
					}, nil
				},
			},
			VDCWriter: &mocks.FakeJobWriter{
				UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
					return nil
				},
			},
		},
	}
	sut.interrupted = true

	_, skipped, err := sut.runJob(job.StartOptions{})

	assert.True(t, skipped)
	assert.Nil(t, err)
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
		JobService: JobService{
			VDCStarter: &mocks.FakeJobStarter{
				StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
					return "1", false, nil
				},
			},
			VDCReader: &mocks.FakeJobReader{
				PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
					return job.Job{ID: id, TimedOut: true}, nil
				},
			},
			VDCStopper: &mocks.FakeJobStopper{
				StopJobFn: func(ctx context.Context, jobID string) (job.Job, error) {
					return job.Job{ID: jobID}, nil
				},
			},
			VDCWriter: &mocks.FakeJobWriter{UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
				return nil
			}},
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
			JobService: JobService{
				VDCStarter: &mocks.FakeJobStarter{
					StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
						return "1", false, nil
					},
				},
				VDCReader: &mocks.FakeJobReader{
					PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
						return job.Job{ID: id, Passed: false}, nil
					},
				},
				VDCStopper: &mocks.FakeJobStopper{
					StopJobFn: func(ctx context.Context, jobID string) (job.Job, error) {
						return job.Job{ID: jobID}, nil
					},
				},
				VDCWriter: &mocks.FakeJobWriter{UploadAssetFn: func(jobID string, fileName string, contentType string, content []byte) error {
					return nil
				}},
			},
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
		JobService: JobService{
			RDCStarter: &mocks.FakeJobStarter{
				StartJobFn: func(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
					return "1", true, nil
				},
			},
			RDCReader: &mocks.FakeJobReader{
				PollJobFn: func(ctx context.Context, id string, interval time.Duration, timeout time.Duration) (job.Job, error) {
					return job.Job{ID: id, TimedOut: true}, nil
				},
			},
		},
	}

	opts := make(chan job.StartOptions)
	results := make(chan result)

	go r.runJobs(opts, results)
	opts <- job.StartOptions{
		DisplayName: "dummy",
		Timeout:     1,
		RealDevice:  true,
	}
	close(opts)
	res := <-results
	assert.Error(t, res.err, "suite 'dummy' has reached timeout")
	assert.True(t, res.job.TimedOut)
}

func TestCheckPathLength(t *testing.T) {
	dir := fs.NewDir(t, "passing",
		fs.WithDir("failing-test",
			fs.WithMode(0755),
			fs.WithDir("bqRamRa7aqyg3mDeaP8zvx7fUs5m5vr74g9ecPyAUkk93MyeETA6hWjyhgsPGtNQS9WEwJpmswcCADYJs7y8t55FsP79TZw7Fy7x",
				fs.WithMode(0755),
				fs.WithDir("dR6y58AjgHCunQ6VtrbbsWyhdMXLtf7xUAvuwmx67sqDpDW2Ln6bYFX6tzK8xufHM9UJWT9KLENTF4UtYehwxbZev59rUtWNbW2k",
					fs.WithMode(0755),
					fs.WithFile("test.spec.js", "dummy-content", fs.WithMode(0644)),
				),
			),
		),
		fs.WithDir("passing-test",
			fs.WithMode(0755),
			fs.WithDir("dir1",
				fs.WithMode(0755),
				fs.WithDir("dir2",
					fs.WithMode(0755),
					fs.WithFile("test.spec.js", "dummy-content", fs.WithMode(0644)),
				),
			),
		),
	)
	defer dir.Remove()

	// Use created dir as referential
	wd, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("failed to change directory to %s: %v", wd, err)
		}
	}()
	if err := os.Chdir(dir.Path()); err != nil {
		t.Errorf("failed to change directory to %s: %v", dir.Path(), err)
	}

	type args struct {
		projectFolder string
		matcher       sauceignore.Matcher
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr error
	}{
		{
			name: "Passing filepath",
			args: args{
				projectFolder: "passing-test",
				matcher:       sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			want:    "",
			wantErr: nil,
		},
		{
			name: "Failing filepath",
			args: args{
				projectFolder: "failing-test",
				matcher:       sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			want:    "failing-test/bqRamRa7aqyg3mDeaP8zvx7fUs5m5vr74g9ecPyAUkk93MyeETA6hWjyhgsPGtNQS9WEwJpmswcCADYJs7y8t55FsP79TZw7Fy7x/dR6y58AjgHCunQ6VtrbbsWyhdMXLtf7xUAvuwmx67sqDpDW2Ln6bYFX6tzK8xufHM9UJWT9KLENTF4UtYehwxbZev59rUtWNbW2k/test.spec.js",
			wantErr: errors.New("path too long"),
		},
		{
			name: "Excluding filepath",
			args: args{
				projectFolder: "failing-test",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{
					{
						P: "bqRamRa7aqyg3mDeaP8zvx7fUs5m5vr74g9ecPyAUkk93MyeETA6hWjyhgsPGtNQS9WEwJpmswcCADYJs7y8t55FsP79TZw7Fy7x",
					},
				}),
			},
			want:    "",
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkPathLength(tt.args.projectFolder, tt.args.matcher)

			assert.Equalf(t, tt.want, got, "checkPathLength(%v, %v)", tt.args.projectFolder, tt.args.matcher)
			assert.Equalf(t, tt.wantErr, err, "checkPathLength(%v, %v)", tt.args.projectFolder, tt.args.matcher)
		})
	}
}
