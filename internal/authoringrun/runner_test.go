package authoringrun

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/retry"
	"github.com/stretchr/testify/assert"
)

func testSauceConfig(concurrency int) config.SauceConfig {
	return config.SauceConfig{Region: "us-west-1", Concurrency: concurrency}
}

// mockAuthoringService implements authoring.Service with function fields so
// each test only needs to set the methods it actually exercises.
type mockAuthoringService struct {
	ListTestCasesFn func(ctx context.Context, opts authoring.ListTestCaseOptions) ([]authoring.TestCase, int, error)
	RunTestCaseFn   func(ctx context.Context, id string, opts authoring.RunTestCaseOptions) (authoring.TestCaseRun, error)
}

func (m *mockAuthoringService) GenerateTestCase(context.Context, authoring.GenerateRequest) (authoring.GenerateTask, error) {
	return authoring.GenerateTask{}, nil
}
func (m *mockAuthoringService) GetGenerateTask(context.Context, string) (authoring.GenerateTask, error) {
	return authoring.GenerateTask{}, nil
}
func (m *mockAuthoringService) GetTestCase(context.Context, string) (authoring.TestCase, error) {
	return authoring.TestCase{}, nil
}
func (m *mockAuthoringService) ListTestCases(ctx context.Context, opts authoring.ListTestCaseOptions) ([]authoring.TestCase, int, error) {
	return m.ListTestCasesFn(ctx, opts)
}
func (m *mockAuthoringService) DeleteTestCase(context.Context, string) error { return nil }
func (m *mockAuthoringService) RunTestCase(ctx context.Context, id string, opts authoring.RunTestCaseOptions) (authoring.TestCaseRun, error) {
	return m.RunTestCaseFn(ctx, id, opts)
}
func (m *mockAuthoringService) ListTestSuites(context.Context, authoring.ListTestSuiteOptions) ([]authoring.TestSuite, error) {
	return nil, nil
}
func (m *mockAuthoringService) GetTestSuite(context.Context, string) (authoring.TestSuite, error) {
	return authoring.TestSuite{}, nil
}
func (m *mockAuthoringService) CreateTestSuite(context.Context, string, []string) (authoring.TestSuite, error) {
	return authoring.TestSuite{}, nil
}
func (m *mockAuthoringService) UpdateTestSuite(context.Context, string, authoring.UpdateTestSuiteOptions) (authoring.TestSuite, error) {
	return authoring.TestSuite{}, nil
}
func (m *mockAuthoringService) DeleteTestSuite(context.Context, string) error { return nil }
func (m *mockAuthoringService) RunTestSuite(context.Context, string, string) (authoring.SuiteRun, error) {
	return authoring.SuiteRun{}, nil
}

// mockJobService implements job.Service with a function field for PollJob
// only, since that's all this runner calls.
type mockJobService struct {
	PollJobFn func(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error)
}

func (m *mockJobService) StartJob(context.Context, job.StartOptions) (job.Job, error) {
	return job.Job{}, nil
}
func (m *mockJobService) StopJob(context.Context, string, bool) (job.Job, error) { return job.Job{}, nil }
func (m *mockJobService) Job(context.Context, string, bool) (job.Job, error)     { return job.Job{}, nil }
func (m *mockJobService) PollJob(ctx context.Context, id string, interval, timeout time.Duration, realDevice bool) (job.Job, error) {
	return m.PollJobFn(ctx, id, interval, timeout, realDevice)
}
func (m *mockJobService) Artifact(context.Context, string, string, bool, retry.Options) ([]byte, error) {
	return nil, nil
}
func (m *mockJobService) ArtifactNames(context.Context, string, bool) ([]string, error) { return nil, nil }
func (m *mockJobService) UploadArtifact(context.Context, string, bool, string, string, []byte) error {
	return nil
}
func (m *mockJobService) DownloadArtifacts(context.Context, job.Job, bool) []string { return nil }

func testCases(n int) []authoring.TestCase {
	var tcs []authoring.TestCase
	for i := 0; i < n; i++ {
		tcs = append(tcs, authoring.TestCase{ID: fmt.Sprintf("tc-%d", i), Name: fmt.Sprintf("test case %d", i)})
	}
	return tcs
}

func TestRunner_RunSuite_AllPass(t *testing.T) {
	auth := &mockAuthoringService{
		ListTestCasesFn: func(_ context.Context, opts authoring.ListTestCaseOptions) ([]authoring.TestCase, int, error) {
			if opts.Skip > 0 {
				return nil, 3, nil
			}
			return testCases(3), 3, nil
		},
		RunTestCaseFn: func(_ context.Context, id string, _ authoring.RunTestCaseOptions) (authoring.TestCaseRun, error) {
			return authoring.TestCaseRun{
				TestCaseID: id,
				Jobs:       []authoring.TestCaseJob{{ID: "job-" + id, SauceJobID: "sauce-" + id}},
			}, nil
		},
	}
	jobs := &mockJobService{
		PollJobFn: func(_ context.Context, id string, _, _ time.Duration, _ bool) (job.Job, error) {
			return job.Job{ID: id, Passed: true, Status: job.StatePassed}, nil
		},
	}

	r := &Runner{
		Project: Project{
			Sauce:  testSauceConfig(2),
			Suites: []Suite{{Name: "regression", TestSuiteID: "suite-1"}},
		},
		Client:     auth,
		JobService: jobs,
	}

	code, err := r.RunProject(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, code)
}

func TestRunner_RunSuite_FailurePropagates(t *testing.T) {
	auth := &mockAuthoringService{
		ListTestCasesFn: func(_ context.Context, opts authoring.ListTestCaseOptions) ([]authoring.TestCase, int, error) {
			if opts.Skip > 0 {
				return nil, 2, nil
			}
			return testCases(2), 2, nil
		},
		RunTestCaseFn: func(_ context.Context, id string, _ authoring.RunTestCaseOptions) (authoring.TestCaseRun, error) {
			return authoring.TestCaseRun{
				TestCaseID: id,
				Jobs:       []authoring.TestCaseJob{{ID: "job-" + id, SauceJobID: "sauce-" + id}},
			}, nil
		},
	}
	jobs := &mockJobService{
		PollJobFn: func(_ context.Context, id string, _, _ time.Duration, _ bool) (job.Job, error) {
			if id == "sauce-tc-0" {
				return job.Job{ID: id, Passed: false, Status: job.StateFailed}, nil
			}
			return job.Job{ID: id, Passed: true, Status: job.StatePassed}, nil
		},
	}

	r := &Runner{
		Project: Project{
			Sauce:  testSauceConfig(2),
			Suites: []Suite{{Name: "regression", TestSuiteID: "suite-1"}},
		},
		Client:     auth,
		JobService: jobs,
		Reporters:  []report.Reporter{},
	}

	code, err := r.RunProject(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, code)
}

func TestRunner_RunSuite_RespectsConcurrencyLimit(t *testing.T) {
	const ccy = 2
	var inFlight int32
	var maxObserved int32

	auth := &mockAuthoringService{
		ListTestCasesFn: func(_ context.Context, opts authoring.ListTestCaseOptions) ([]authoring.TestCase, int, error) {
			if opts.Skip > 0 {
				return nil, 6, nil
			}
			return testCases(6), 6, nil
		},
		RunTestCaseFn: func(_ context.Context, id string, _ authoring.RunTestCaseOptions) (authoring.TestCaseRun, error) {
			cur := atomic.AddInt32(&inFlight, 1)
			defer atomic.AddInt32(&inFlight, -1)
			for {
				max := atomic.LoadInt32(&maxObserved)
				if cur <= max || atomic.CompareAndSwapInt32(&maxObserved, max, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			return authoring.TestCaseRun{
				TestCaseID: id,
				Jobs:       []authoring.TestCaseJob{{ID: "job-" + id, SauceJobID: "sauce-" + id}},
			}, nil
		},
	}
	jobs := &mockJobService{
		PollJobFn: func(_ context.Context, id string, _, _ time.Duration, _ bool) (job.Job, error) {
			return job.Job{ID: id, Passed: true, Status: job.StatePassed}, nil
		},
	}

	r := &Runner{
		Project: Project{
			Sauce:  testSauceConfig(ccy),
			Suites: []Suite{{Name: "regression", TestSuiteID: "suite-1"}},
		},
		Client:     auth,
		JobService: jobs,
	}

	code, err := r.RunProject(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.LessOrEqual(t, int(atomic.LoadInt32(&maxObserved)), ccy, "never more than the configured concurrency should run at once")
	assert.Equal(t, int32(ccy), atomic.LoadInt32(&maxObserved), "with 6 test cases and concurrency 2, it should actually reach the concurrency limit")
}

func TestRunner_RunSuite_NoTestCases(t *testing.T) {
	called := false
	auth := &mockAuthoringService{
		ListTestCasesFn: func(context.Context, authoring.ListTestCaseOptions) ([]authoring.TestCase, int, error) {
			return nil, 0, nil
		},
		RunTestCaseFn: func(context.Context, string, authoring.RunTestCaseOptions) (authoring.TestCaseRun, error) {
			called = true
			return authoring.TestCaseRun{}, nil
		},
	}

	r := &Runner{
		Project: Project{
			Sauce:  testSauceConfig(2),
			Suites: []Suite{{Name: "regression", TestSuiteID: "suite-1"}},
		},
		Client: auth,
	}

	code, err := r.RunProject(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.False(t, called, "RunTestCase should never be called when the suite has no test cases")
}
