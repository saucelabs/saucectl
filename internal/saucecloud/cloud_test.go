package saucecloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/saucecloud/zip"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestSignalDetection(t *testing.T) {
	r := CloudRunner{JobService: JobService{VDCStopper: &mocks.FakeJobStopper{}}}
	assert.False(t, r.interrupted)
	c := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(c)

	c <- syscall.SIGINT

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()

	// Wait for interrupt to be processed, as it happens asynchronously.
	for {
		select {
		case <-deadline.C:
			assert.True(t, r.interrupted)
			return
		default:
			if r.interrupted {
				return
			}
			time.Sleep(1 * time.Nanosecond) // allow context switch
		}
	}
}

func TestSignalDetectionExit(t *testing.T) {
	if os.Getenv("FORCE_EXIT_TEST") == "1" {
		r := CloudRunner{JobService: JobService{VDCStopper: &mocks.FakeJobStopper{}}}
		assert.False(t, r.interrupted)
		c := r.registerSkipSuitesOnSignal()
		defer unregisterSignalCapture(c)

		c <- syscall.SIGINT

		deadline := time.NewTimer(3 * time.Second)
		defer deadline.Stop()

		// Wait for interrupt to be processed, as it happens asynchronously.
	loop:
		for {
			select {
			case <-deadline.C:
				return
			default:
				if r.interrupted {
					break loop
				}
				time.Sleep(1 * time.Nanosecond) // allow context switch
			}
		}

		c <- syscall.SIGINT

		// Process should get killed due to double interrupt. If this doesn't happen, the test will exit cleanly
		// which will be caught by the original process of the test, which expects an exit code of 1.
		time.Sleep(3 * time.Second)
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
			Retrier: &retry.SauceReportRetrier{},
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
		assert.Equal(t, len(res.attempts), tt.wantAttempts)
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
			RDCStopper: &mocks.FakeJobStopper{
				StopJobFn: func(ctx context.Context, id string) (job.Job, error) {
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
	assert.Error(t, res.err)
	assert.True(t, res.job.TimedOut)
}

func TestCloudRunner_archiveNodeModules(t *testing.T) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(tempDir)

	projectsDir := fs.NewDir(t, "project",
		fs.WithDir("has-mods",
			fs.WithDir("node_modules",
				fs.WithDir("mod1",
					fs.WithFile("package.json", "{}"),
				),
			),
		),
		fs.WithDir("no-mods"),
		fs.WithDir("empty-mods",
			fs.WithDir("node_modules"),
		),
	)
	defer projectsDir.Remove()

	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get the current working dir: %v", err)
	}

	if err := os.Chdir(projectsDir.Path()); err != nil {
		t.Errorf("Failed to change the current working dir: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("Failed to change the current working dir back to original: %v", err)
		}
	}()

	type fields struct {
		NPMDependencies []string
	}
	type args struct {
		tempDir string
		rootDir string
		matcher sauceignore.Matcher
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			"want to include mods, but node_modules does not exist",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "no-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			"",
			func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.EqualError(t, err, "unable to access 'node_modules' folder, but you have npm dependencies defined in your configuration; ensure that the folder exists and is accessible", args)
			},
		},
		{
			"have and want mods, but mods are ignored",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "has-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{sauceignore.NewPattern("/has-mods/node_modules")}),
			},
			"",
			func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.EqualError(t, err, "'node_modules' is ignored by sauceignore, but you have npm dependencies defined in your project; please remove 'node_modules' from your sauceignore file", args)
			},
		},
		{
			"have mods, don't want them and they are ignored",
			fields{
				NPMDependencies: []string{}, // no mods selected, because we don't want any
			},
			args{
				tempDir: tempDir,
				rootDir: "has-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{sauceignore.NewPattern("/has-mods/node_modules")}),
			},
			"",
			assert.NoError,
		},
		{
			"no mods wanted and no mods exist",
			fields{
				NPMDependencies: []string{},
			},
			args{
				tempDir: tempDir,
				rootDir: "no-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			"",
			assert.NoError,
		},
		{
			"has and wants mods (happy path)",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "has-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			filepath.Join(tempDir, "node_modules.zip"),
			assert.NoError,
		},
		{
			"want mods, but node_modules folder is empty",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "empty-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			"",
			func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.EqualError(t, err, "unable to find required dependencies; please check 'node_modules' folder and make sure the dependencies exist", args)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CloudRunner{
				NPMDependencies: tt.fields.NPMDependencies,
			}
			got, err := zip.ArchiveNodeModules(tt.args.tempDir, tt.args.rootDir, tt.args.matcher, r.NPMDependencies)
			if !tt.wantErr(t, err, fmt.Sprintf("archiveNodeModules(%v, %v, %v)", tt.args.tempDir, tt.args.rootDir, tt.args.matcher)) {
				return
			}
			assert.Equalf(t, tt.want, got, "archiveNodeModules(%v, %v, %v)", tt.args.tempDir, tt.args.rootDir, tt.args.matcher)
		})
	}
}

func Test_arrayContains(t *testing.T) {
	type args struct {
		list []string
		want string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Empty set",
			args: args{
				list: []string{},
				want: "value",
			},
			want: false,
		},
		{
			name: "Complete set - false",
			args: args{
				list: []string{"val1", "val2", "val3"},
				want: "value",
			},
			want: false,
		},
		{
			name: "Found",
			args: args{
				list: []string{"val1", "val2", "val3"},
				want: "val1",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, arrayContains(tt.args.list, tt.args.want), "arrayContains(%v, %v)", tt.args.list, tt.args.want)
		})
	}
}

func TestCloudRunner_loadSauceTestReport(t *testing.T) {
	type args struct {
		jobID string
		isRDC bool
	}
	type fields struct {
		GetJobAssetFileNamesFn   func(ctx context.Context, jobID string) ([]string, error)
		GetJobAssetFileContentFn func(ctx context.Context, jobID, fileName string) ([]byte, error)
	}
	tests := []struct {
		name    string
		args    args
		fields  fields
		want    saucereport.SauceReport
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Complete unmarshall",
			args: args{
				jobID: "test1",
				isRDC: false,
			},
			fields: fields{
				GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
					return []string{saucereport.FileName}, nil
				},
				GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
					if fileName == saucereport.FileName {
						return []byte(`{"status":"failed","attachments":[],"suites":[{"name":"cypress/e2e/examples/actions.cy.js","status":"failed","metadata":{},"suites":[{"name":"Actions","status":"failed","metadata":{},"suites":[],"attachments":[],"tests":[{"name":".type() - type into a DOM element","status":"passed","startTime":"2022-12-22T10:10:11.083Z","duration":1802,"metadata":{},"output":null,"attachments":[],"code":{"lines":["() => {","    // https://on.cypress.io/type","    cy.get('.action-email').type('fake@email.com').should('have.value', 'fake@email.com');","  }"]},"videoTimestamp":26.083},{"name":".type() - type into a wrong DOM element","status":"failed","startTime":"2022-12-22T10:10:12.907Z","duration":5010,"metadata":{},"output":"AssertionError: Timed out retrying after 4000ms: expected '<input#email1.form-control.action-email>' to have value 'wrongy@email.com', but the value was 'fake@email.com'\n\n  11 |     // https://on.cypress.io/type\n  12 |     cy.get('.action-email')\n> 13 |         .type('fake@email.com').should('have.value', 'wrongy@email.com')\n     |                                 ^\n  14 |   })\n  15 | })\n  16 | ","attachments":[{"name":"screenshot","path":"Actions -- .type() - type into a wrong DOM element (failed).png","contentType":"image/png"}],"code":{"lines":["() => {","    // https://on.cypress.io/type","    cy.get('.action-email').type('fake@email.com').should('have.value', 'wrongy@email.com');","  }"]},"videoTimestamp":27.907}]}],"attachments":[],"tests":[]}],"metadata":{}}`), nil
					}
					return []byte{}, errors.New("not-found")
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return err == nil
			},
			want: saucereport.SauceReport{
				Status:      saucereport.StatusFailed,
				Attachments: []saucereport.Attachment{},
				Suites: []saucereport.Suite{
					{
						Name:        "cypress/e2e/examples/actions.cy.js",
						Status:      saucereport.StatusFailed,
						Attachments: []saucereport.Attachment{},
						Metadata:    saucereport.Metadata{},
						Tests:       []saucereport.Test{},
						Suites: []saucereport.Suite{
							{
								Name:        "Actions",
								Status:      saucereport.StatusFailed,
								Attachments: []saucereport.Attachment{},
								Suites:      []saucereport.Suite{},
								Metadata:    saucereport.Metadata{},
								Tests: []saucereport.Test{
									{
										Name:      ".type() - type into a DOM element",
										Status:    saucereport.StatusPassed,
										StartTime: time.Date(2022, 12, 22, 10, 10, 11, 83000000, time.UTC),
										Duration:  1802,
										Metadata:  saucereport.Metadata{},
										Code: saucereport.Code{
											Lines: []string{
												"() => {",
												"    // https://on.cypress.io/type",
												"    cy.get('.action-email').type('fake@email.com').should('have.value', 'fake@email.com');",
												"  }",
											},
										},
										VideoTimestamp: 26.083,
										Attachments:    []saucereport.Attachment{},
									},
									{
										Name:      ".type() - type into a wrong DOM element",
										Status:    saucereport.StatusFailed,
										StartTime: time.Date(2022, 12, 22, 10, 10, 12, 907000000, time.UTC),
										Duration:  5010,
										Output:    "AssertionError: Timed out retrying after 4000ms: expected '<input#email1.form-control.action-email>' to have value 'wrongy@email.com', but the value was 'fake@email.com'\n\n  11 |     // https://on.cypress.io/type\n  12 |     cy.get('.action-email')\n> 13 |         .type('fake@email.com').should('have.value', 'wrongy@email.com')\n     |                                 ^\n  14 |   })\n  15 | })\n  16 | ",
										Attachments: []saucereport.Attachment{
											{
												Name:        "screenshot",
												Path:        "Actions -- .type() - type into a wrong DOM element (failed).png",
												ContentType: "image/png",
											},
										},
										Metadata: saucereport.Metadata{},
										Code: saucereport.Code{
											Lines: []string{
												"() => {",
												"    // https://on.cypress.io/type",
												"    cy.get('.action-email').type('fake@email.com').should('have.value', 'wrongy@email.com');",
												"  }",
											},
										},
										VideoTimestamp: 27.907,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CloudRunner{
				JobService: JobService{
					VDCReader: &mocks.FakeJobReader{
						GetJobAssetFileNamesFn:   tt.fields.GetJobAssetFileNamesFn,
						GetJobAssetFileContentFn: tt.fields.GetJobAssetFileContentFn,
					},
				},
			}
			got, err := r.loadSauceTestReport(tt.args.jobID, tt.args.isRDC)
			if !tt.wantErr(t, err, fmt.Sprintf("loadSauceTestReport(%v, %v)", tt.args.jobID, tt.args.isRDC)) {
				return
			}
			assert.Equalf(t, tt.want, got, "loadSauceTestReport(%v, %v)", tt.args.jobID, tt.args.isRDC)
		})
	}
}

func TestCloudRunner_loadJUnitReport(t *testing.T) {
	type args struct {
		jobID string
		isRDC bool
	}
	type fields struct {
		GetJobAssetFileNamesFn   func(ctx context.Context, jobID string) ([]string, error)
		GetJobAssetFileContentFn func(ctx context.Context, jobID, fileName string) ([]byte, error)
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    junit.TestSuites
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Unmarshall XML",
			fields: fields{
				GetJobAssetFileNamesFn: func(ctx context.Context, jobID string) ([]string, error) {
					return []string{junit.FileName}, nil
				},
				GetJobAssetFileContentFn: func(ctx context.Context, jobID, fileName string) ([]byte, error) {
					if fileName == junit.FileName {
						return []byte(`<?xml version="1.0" encoding="utf-8"?><testsuite package="com.saucelabs.mydemoapp.android" tests="7" time="52.056"><testcase classname="com.saucelabs.mydemoapp.android.view.activities.DashboardToCheckout" name="dashboardProductTest" status="success"/><testcase classname="com.saucelabs.mydemoapp.android.view.activities.LoginTest" name="succesfulLoginTest" status="success"/><testcase classname="com.saucelabs.mydemoapp.android.view.activities.LoginTest" name="noUsernameLoginTest" status="success"/><testcase classname="com.saucelabs.mydemoapp.android.view.activities.LoginTest" name="noPasswordLoginTest" status="success"/><testcase classname="com.saucelabs.mydemoapp.android.view.activities.LoginTest" name="noCredentialLoginTest" status="success"/><testcase classname="com.saucelabs.mydemoapp.android.view.activities.WebViewTest" name="webViewTest" status="success"/><testcase classname="com.saucelabs.mydemoapp.android.view.activities.WebViewTest" name="withoutUrlTest" status="success"/><system-out>INSTRUMENTATION_STATUS: class=com.saucelabs.mydemoapp.android.view.activities.DashboardToCheckout</system-out></testsuite>`), nil
					}
					return []byte{}, errors.New("not-found")
				},
			},
			args: args{
				jobID: "dummy-jobID",
				isRDC: false,
			},
			want: junit.TestSuites{
				TestSuites: []junit.TestSuite{
					{
						Package: "com.saucelabs.mydemoapp.android",
						Tests:   7,
						Time:    "52.056",
						TestCases: []junit.TestCase{
							{
								ClassName: "com.saucelabs.mydemoapp.android.view.activities.DashboardToCheckout",
								Name:      "dashboardProductTest",
								Status:    "success",
							},
							{
								ClassName: "com.saucelabs.mydemoapp.android.view.activities.LoginTest",
								Name:      "succesfulLoginTest",
								Status:    "success",
							},
							{
								ClassName: "com.saucelabs.mydemoapp.android.view.activities.LoginTest",
								Name:      "noUsernameLoginTest",
								Status:    "success",
							},
							{
								ClassName: "com.saucelabs.mydemoapp.android.view.activities.LoginTest",
								Name:      "noPasswordLoginTest",
								Status:    "success",
							},
							{
								ClassName: "com.saucelabs.mydemoapp.android.view.activities.LoginTest",
								Name:      "noCredentialLoginTest",
								Status:    "success",
							},
							{
								ClassName: "com.saucelabs.mydemoapp.android.view.activities.WebViewTest",
								Name:      "webViewTest",
								Status:    "success",
							},
							{
								ClassName: "com.saucelabs.mydemoapp.android.view.activities.WebViewTest",
								Name:      "withoutUrlTest",
								Status:    "success",
							},
						},
						SystemOut: "INSTRUMENTATION_STATUS: class=com.saucelabs.mydemoapp.android.view.activities.DashboardToCheckout",
					},
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return err == nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CloudRunner{
				JobService: JobService{
					VDCReader: &mocks.FakeJobReader{
						GetJobAssetFileNamesFn:   tt.fields.GetJobAssetFileNamesFn,
						GetJobAssetFileContentFn: tt.fields.GetJobAssetFileContentFn,
					},
				},
			}
			got, err := r.loadJUnitReport(tt.args.jobID, tt.args.isRDC)
			if !tt.wantErr(t, err, fmt.Sprintf("loadJUnitReport(%v, %v)", tt.args.jobID, tt.args.isRDC)) {
				return
			}
			assert.Equalf(t, tt.want, got, "loadJUnitReport(%v, %v)", tt.args.jobID, tt.args.isRDC)
		})
	}
}
