package authoring

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

// fakeService is a package-local fake for the two services the runner uses.
// It cannot live in internal/mocks: that package imports this one, and a
// test here importing it would form a cycle.
type fakeService struct {
	TestCaseService
	TestSuiteService
	getTestCase    func(ctx context.Context, id string) (TestCase, error)
	listTestCases  func(ctx context.Context, opts ListTestCasesOptions) (List[TestCase], error)
	listTestSuites func(ctx context.Context, opts ListTestSuitesOptions) (List[TestSuite], error)
	runTestCase    func(ctx context.Context, id, revisionID string, opts RunOptions) (Run, error)
	getRun         func(ctx context.Context, testCaseID, runID string) (Run, error)
}

func (f *fakeService) GetTestCase(ctx context.Context, id string) (TestCase, error) {
	return f.getTestCase(ctx, id)
}

func (f *fakeService) ListTestCases(ctx context.Context, opts ListTestCasesOptions) (List[TestCase], error) {
	return f.listTestCases(ctx, opts)
}

func (f *fakeService) ListTestSuites(ctx context.Context, opts ListTestSuitesOptions) (List[TestSuite], error) {
	return f.listTestSuites(ctx, opts)
}

func (f *fakeService) RunTestCase(ctx context.Context, id, revisionID string, opts RunOptions) (Run, error) {
	return f.runTestCase(ctx, id, revisionID, opts)
}

func (f *fakeService) GetRun(ctx context.Context, testCaseID, runID string) (Run, error) {
	return f.getRun(ctx, testCaseID, runID)
}

// captureReporter records what the runner reports.
type captureReporter struct {
	mu        sync.Mutex
	results   []report.TestResult
	rendered  bool
	wantJUnit bool
}

func (c *captureReporter) Add(t report.TestResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, t)
}
func (c *captureReporter) Render() { c.rendered = true }
func (c *captureReporter) Reset()  { c.results = nil }
func (c *captureReporter) ArtifactRequirements() []report.ArtifactType {
	if c.wantJUnit {
		return []report.ArtifactType{report.JUnitArtifact}
	}
	return nil
}

// fakeBuilds resolves every job to one build URL.
type fakeBuilds struct{ calls int32 }

func (f *fakeBuilds) GetBuild(_ context.Context, opts build.GetBuildOptions) (build.Build, error) {
	atomic.AddInt32(&f.calls, 1)
	return build.Build{ID: "b", URL: "https://app.saucelabs.com/builds/" + string(opts.Source) + "/b"}, nil
}
func (f *fakeBuilds) ListBuilds(context.Context, build.ListBuildsOptions) ([]build.Build, error) {
	return nil, nil
}

// fakeTunnels records the tunnel readiness check.
type fakeTunnels struct{ name, owner string }

func (f *fakeTunnels) IsTunnelRunning(_ context.Context, id, owner string, _ tunnel.Filter, _ time.Duration) error {
	f.name, f.owner = id, owner
	return nil
}

// fakeDownloader records the jobs handed to it.
type fakeDownloader struct {
	mu   sync.Mutex
	jobs []job.Job
}

func (f *fakeDownloader) DownloadArtifacts(_ context.Context, j job.Job, _ bool) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, j)
	return []string{"artifacts/" + j.ID + "/video.mp4"}
}

func newProject(suites ...Suite) Project {
	p := Project{
		Sauce:  config.SauceConfig{Region: "us-west-1", Concurrency: 2, Metadata: config.Metadata{Build: "nightly"}},
		Suites: suites,
	}
	SetDefaults(&p)
	return p
}

func chromeJob(success *bool) RunJob {
	return RunJob{ID: "j1", SauceJobID: "sauce-1", Name: "n", Success: success,
		Target: Target{Capabilities: map[string]any{"browserName": "chrome", "browserVersion": "latest", "platformName": "Windows 11"}}}
}

func newRunner(svc *fakeService, rep *captureReporter, p Project) *Runner {
	return &Runner{Project: p, TestCases: svc, TestSuites: svc, Reporters: []report.Reporter{rep}, PollInterval: time.Millisecond}
}

func TestRunner_SynchronousStartNeverPolls(t *testing.T) {
	yes := true
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(&yes)}}, nil
		},
		getRun: func(context.Context, string, string) (Run, error) {
			t.Fatal("a terminal start response must not be polled")
			return Run{}, nil
		},
	}
	rep := &captureReporter{}
	r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}}))

	code, err := r.RunProject(context.Background())
	if err != nil || code != 0 {
		t.Fatalf("exit %d, err %v", code, err)
	}
	if len(rep.results) != 1 || rep.results[0].Status != job.StatePassed || !rep.rendered {
		t.Errorf("results = %+v rendered=%v", rep.results, rep.rendered)
	}
	res := rep.results[0]
	if res.Name != "s - case" || res.Browser != "chrome latest" || res.Platform != "Windows 11" || res.URL != "/tests/sauce-1" || res.RDC {
		t.Errorf("result fields = %+v", res)
	}
}

func TestRunner_PollsUntilDone(t *testing.T) {
	yes := true
	var polls int32
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(nil)}}, nil
		},
		getRun: func(_ context.Context, testCaseID, runID string) (Run, error) {
			if testCaseID != "tc1" || runID != "run" {
				t.Errorf("polled with %s/%s; must use the run's own testCaseId", testCaseID, runID)
			}
			n := atomic.AddInt32(&polls, 1)
			j := chromeJob(nil)
			if n >= 3 {
				j.Success = &yes
			}
			return Run{ID: runID, TestCaseID: testCaseID, Jobs: []RunJob{j}}, nil
		},
	}
	rep := &captureReporter{}
	r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}}))

	code, _ := r.RunProject(context.Background())
	if code != 0 || atomic.LoadInt32(&polls) != 3 {
		t.Errorf("exit %d after %d polls", code, polls)
	}
}

func TestRunner_MultiTargetReportsOnePerJob(t *testing.T) {
	yes, no := true, false
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{
				chromeJob(&yes),
				{ID: "j2", SauceJobID: "sauce-2", Success: &no, Error: "assertion failed",
					Target: Target{IsRDC: true, Capabilities: map[string]any{"platformName": "Android", "appium:platformVersion": "16", "appium:deviceName": "Google Pixel 9"}}},
			}}, nil
		},
	}
	rep := &captureReporter{}
	r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}}))

	code, _ := r.RunProject(context.Background())
	if code != 1 {
		t.Errorf("exit = %d, want 1 because one job failed", code)
	}
	if len(rep.results) != 2 {
		t.Fatalf("got %d results, want one per job", len(rep.results))
	}
	var rdc report.TestResult
	for _, res := range rep.results {
		if res.RDC {
			rdc = res
		}
	}
	if rdc.Status != job.StateFailed || rdc.DeviceName != "Google Pixel 9" || rdc.Platform != "Android 16" {
		t.Errorf("rdc result = %+v", rdc)
	}
}

func TestRunner_TimeoutSetsTimedOut(t *testing.T) {
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(nil)}}, nil
		},
		getRun: func(_ context.Context, tc, id string) (Run, error) {
			return Run{ID: id, TestCaseID: tc, Jobs: []RunJob{chromeJob(nil)}}, nil
		},
	}
	rep := &captureReporter{}
	r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}, Timeout: 30 * time.Millisecond}))

	code, _ := r.RunProject(context.Background())
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	res := rep.results[0]
	if !res.TimedOut || res.Status != job.StateInProgress {
		t.Errorf("a timed-out run must be in progress AND TimedOut so the table counts it as an error: %+v", res)
	}
}

func TestRunner_StartFailure(t *testing.T) {
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(context.Context, string, string, RunOptions) (Run, error) {
			return Run{}, &APIError{HTTPStatus: 400, Code: "SC_TUNNEL_NOT_FOUND", Detail: "Sauce Connect tunnel does not exist."}
		},
	}
	rep := &captureReporter{wantJUnit: true}
	r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}}))

	code, _ := r.RunProject(context.Background())
	if code != 1 || len(rep.results) != 1 || rep.results[0].Status != job.StateFailed {
		t.Fatalf("exit %d results %+v", code, rep.results)
	}
	suites := rep.results[0].Attempts[0].TestSuites.TestSuites
	if len(suites) != 1 || len(suites[0].TestCases) != 1 || suites[0].TestCases[0].Failure == nil {
		t.Fatalf("junit synthesis missing for a start failure: %+v", suites)
	}
	if !strings.Contains(suites[0].TestCases[0].Failure.Message, "SC_TUNNEL_NOT_FOUND") {
		t.Errorf("failure message = %q", suites[0].TestCases[0].Failure.Message)
	}
}

func TestRunner_Async(t *testing.T) {
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(nil)}}, nil
		},
		getRun: func(context.Context, string, string) (Run, error) {
			t.Fatal("async must not poll")
			return Run{}, nil
		},
	}
	rep := &captureReporter{}
	r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}}))
	r.Async = true
	r.Builds = &fakeBuilds{}

	code, _ := r.RunProject(context.Background())
	if code != 0 || rep.results[0].Status != job.StateInProgress || rep.results[0].BuildURL != "" {
		t.Errorf("exit %d results %+v", code, rep.results)
	}
}

func TestRunner_ConcurrencyCeiling(t *testing.T) {
	yes := true
	var inFlight, maxInFlight int32
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: id}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			n := atomic.AddInt32(&inFlight, 1)
			for {
				m := atomic.LoadInt32(&maxInFlight)
				if n <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			return Run{ID: "run-" + id, TestCaseID: id, Jobs: []RunJob{chromeJob(&yes)}}, nil
		},
	}
	rep := &captureReporter{}
	p := newProject(Suite{Name: "s", TestCases: []string{"a", "b", "c", "d", "e"}})
	p.Sauce.Concurrency = 2
	r := newRunner(svc, rep, p)

	code, _ := r.RunProject(context.Background())
	if code != 0 || len(rep.results) != 5 {
		t.Fatalf("exit %d, %d results", code, len(rep.results))
	}
	if maxInFlight > 2 {
		t.Errorf("max in flight = %d, want <= 2 (SC-011)", maxInFlight)
	}
}

func TestRunner_ContextCancellation(t *testing.T) {
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(nil)}}, nil
		},
		getRun: func(_ context.Context, tc, id string) (Run, error) {
			return Run{ID: id, TestCaseID: tc, Jobs: []RunJob{chromeJob(nil)}}, nil
		},
	}
	rep := &captureReporter{}
	r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}, Timeout: time.Hour}))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	code, _ := r.RunProject(ctx)
	if time.Since(start) > 2*time.Second {
		t.Error("cancellation did not stop polling promptly")
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1 on interruption", code)
	}
	res := rep.results[0]
	if res.Status != job.StateInProgress || res.TimedOut {
		t.Errorf("an interrupted run is still running remotely, not timed out: %+v", res)
	}
}

func TestRunner_JUnitSynthesisAndArtifactsAndBuildLink(t *testing.T) {
	no := false
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "Login"}, nil },
		runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
			j := chromeJob(&no)
			j.Error = "element not found"
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{j}}, nil
		},
	}
	rep := &captureReporter{wantJUnit: true}
	dl := &fakeDownloader{}
	r := newRunner(svc, rep, newProject(Suite{Name: "Smoke", TestCases: []string{"tc1"}}))
	r.Artifacts = dl
	r.Builds = &fakeBuilds{}

	code, _ := r.RunProject(context.Background())
	if code != 1 {
		t.Errorf("exit = %d", code)
	}
	res := rep.results[0]

	ts := res.Attempts[0].TestSuites.TestSuites
	if len(ts) != 1 || ts[0].Tests != 1 || ts[0].Failures != 1 || len(ts[0].TestCases) != 1 {
		t.Fatalf("synthesized junit = %+v", ts)
	}
	tc := ts[0].TestCases[0]
	if tc.Name != "Login" || tc.ClassName != "Smoke" || tc.Failure == nil || tc.Failure.Message != "element not found" {
		t.Errorf("junit test case = %+v", tc)
	}

	if res.BuildURL != "https://app.saucelabs.com/builds/vdc/b" {
		t.Errorf("build url = %q", res.BuildURL)
	}
	if len(dl.jobs) != 1 || dl.jobs[0].ID != "sauce-1" || dl.jobs[0].Status != job.StateFailed || dl.jobs[0].Passed || dl.jobs[0].TimedOut {
		t.Errorf("downloader job = %+v; Status/Passed/TimedOut drive skipDownload", dl.jobs)
	}
	if len(res.Artifacts) != 1 || res.Artifacts[0].FilePath != "artifacts/sauce-1/video.mp4" {
		t.Errorf("artifacts = %+v", res.Artifacts)
	}
}

func TestRunner_RunOptionsCarryConfig(t *testing.T) {
	yes := true
	var got RunOptions
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id}, nil },
		runTestCase: func(_ context.Context, id, rev string, opts RunOptions) (Run, error) {
			got = opts
			if rev != "" {
				t.Errorf("revision = %q, want latest", rev)
			}
			return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(&yes)}}, nil
		},
	}
	p := newProject(Suite{Name: "s", TestCases: []string{"tc1"}, Targets: []Target{{Capabilities: map[string]any{"browserName": "firefox"}}}})
	p.Sauce.Tunnel.Name = "my-tunnel"
	SetDefaults(&p)
	tunnels := &fakeTunnels{}
	r := newRunner(svc, &captureReporter{}, p)
	r.Tunnels = tunnels

	code, err := r.RunProject(context.Background())
	if err != nil || code != 0 {
		t.Fatalf("exit %d, err %v", code, err)
	}
	if tunnels.name != "my-tunnel" {
		t.Errorf("tunnel readiness was not checked for %q", "my-tunnel")
	}
	if got.BuildName != "nightly" || got.TunnelName != "my-tunnel" || len(got.Targets) != 1 || got.Targets[0].Capabilities["browserName"] != "firefox" {
		t.Errorf("run options = %+v", got)
	}
}

func TestRunner_TransientAndFatalPollErrors(t *testing.T) {
	yes := true
	t.Run("5xx is retried", func(t *testing.T) {
		var polls int32
		svc := &fakeService{
			getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id}, nil },
			runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
				return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(nil)}}, nil
			},
			getRun: func(_ context.Context, tc, id string) (Run, error) {
				if atomic.AddInt32(&polls, 1) == 1 {
					return Run{}, &APIError{HTTPStatus: 503, Detail: "unavailable"}
				}
				return Run{ID: id, TestCaseID: tc, Jobs: []RunJob{chromeJob(&yes)}}, nil
			},
		}
		r := newRunner(svc, &captureReporter{}, newProject(Suite{Name: "s", TestCases: []string{"tc1"}}))
		if code, _ := r.RunProject(context.Background()); code != 0 {
			t.Errorf("exit = %d", code)
		}
	})
	t.Run("403 is fatal", func(t *testing.T) {
		svc := &fakeService{
			getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id}, nil },
			runTestCase: func(_ context.Context, id, _ string, _ RunOptions) (Run, error) {
				return Run{ID: "run", TestCaseID: id, Jobs: []RunJob{chromeJob(nil)}}, nil
			},
			getRun: func(context.Context, string, string) (Run, error) {
				return Run{}, &APIError{HTTPStatus: 403, Code: "UNAUTHORIZED"}
			},
		}
		rep := &captureReporter{}
		r := newRunner(svc, rep, newProject(Suite{Name: "s", TestCases: []string{"tc1"}, Timeout: time.Hour}))
		start := time.Now()
		code, _ := r.RunProject(context.Background())
		if code != 1 || time.Since(start) > time.Second {
			t.Errorf("exit = %d after %v; a 4xx must fail fast", code, time.Since(start))
		}
	})
}

func TestRunner_ResolveTestCases(t *testing.T) {
	suites := []TestSuite{{ID: "demo", Name: "Demo"}, {ID: "demo-suite", Name: "Demo Suite"}, {ID: "dup1", Name: "Dup"}, {ID: "dup2", Name: "Dup"}}
	var listedSuiteIDs []string
	svc := &fakeService{
		listTestSuites: func(_ context.Context, opts ListTestSuitesOptions) (List[TestSuite], error) {
			// The service's search is a substring match: everything containing
			// the term comes back and the exact match must be made here.
			var out []TestSuite
			for _, s := range suites {
				if strings.Contains(strings.ToLower(s.Name), strings.ToLower(opts.Search)) {
					out = append(out, s)
				}
			}
			return List[TestSuite]{Items: out, Total: len(out)}, nil
		},
		listTestCases: func(_ context.Context, opts ListTestCasesOptions) (List[TestCase], error) {
			listedSuiteIDs = opts.TestSuiteIDs
			if opts.Skip > 0 {
				return List[TestCase]{Total: 2}, nil
			}
			return List[TestCase]{Items: []TestCase{{ID: "a"}, {ID: "b"}}, Total: 2}, nil
		},
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "n-" + id}, nil },
	}

	t.Run("exact name among substring matches", func(t *testing.T) {
		r := newRunner(svc, &captureReporter{}, newProject(Suite{Name: "s", TestSuiteName: "Demo"}))
		cases, err := r.ResolveTestCases(context.Background())
		if err != nil || len(cases) != 2 || listedSuiteIDs[0] != "demo" {
			t.Errorf("cases=%v err=%v listed=%v", cases, err, listedSuiteIDs)
		}
	})
	t.Run("ambiguous name", func(t *testing.T) {
		r := newRunner(svc, &captureReporter{}, newProject(Suite{Name: "s", TestSuiteName: "Dup"}))
		if _, err := r.ResolveTestCases(context.Background()); err == nil || !strings.Contains(err.Error(), "2 test suites") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("unknown name", func(t *testing.T) {
		r := newRunner(svc, &captureReporter{}, newProject(Suite{Name: "s", TestSuiteName: "Nope"}))
		if _, err := r.ResolveTestCases(context.Background()); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("by id and by explicit cases", func(t *testing.T) {
		r := newRunner(svc, &captureReporter{}, newProject(
			Suite{Name: "by-id", TestSuiteID: "xyz", Tags: []string{"smoke"}},
			Suite{Name: "explicit", TestCases: []string{"c1"}},
		))
		cases, err := r.ResolveTestCases(context.Background())
		if err != nil || len(cases) != 3 {
			t.Fatalf("cases=%v err=%v", cases, err)
		}
		if listedSuiteIDs[0] != "xyz" || cases[2].TestCase.Name != "n-c1" || cases[2].Suite.Name != "explicit" {
			t.Errorf("resolution wrong: %v %+v", listedSuiteIDs, cases[2])
		}
	})
	t.Run("missing explicit case is an error", func(t *testing.T) {
		bad := &fakeService{getTestCase: func(context.Context, string) (TestCase, error) {
			return TestCase{}, &APIError{HTTPStatus: 404, Code: "TEST_CASE_NOT_FOUND"}
		}}
		r := newRunner(bad, &captureReporter{}, newProject(Suite{Name: "s", TestCases: []string{"nope"}}))
		if _, err := r.ResolveTestCases(context.Background()); !errors.Is(err, ErrTestCaseNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRunner_DryRunStartsNothing(t *testing.T) {
	svc := &fakeService{
		getTestCase: func(_ context.Context, id string) (TestCase, error) { return TestCase{ID: id, Name: "case"}, nil },
		runTestCase: func(context.Context, string, string, RunOptions) (Run, error) {
			t.Fatal("dry run must not start a run")
			return Run{}, nil
		},
	}
	p := newProject(Suite{Name: "s", TestCases: []string{"tc1"}})
	p.DryRun = true
	rep := &captureReporter{}
	r := newRunner(svc, rep, p)
	code, err := r.RunProject(context.Background())
	if err != nil || code != 0 || len(rep.results) != 0 {
		t.Errorf("exit %d err %v results %d", code, err, len(rep.results))
	}
}
