package apitest

import (
	"context"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/config"

	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

type MockAPITester struct {
	GetProjectFn        func(ctx context.Context, hookID string) (ProjectMeta, error)
	GetEventResultFn    func(ctx context.Context, hookID string, eventID string) (TestResult, error)
	GetTestFn           func(ctx context.Context, hookID string, testID string) (Test, error)
	GetProjectsFn       func(ctx context.Context) ([]ProjectMeta, error)
	GetHooksFn          func(ctx context.Context, projectID string) ([]Hook, error)
	RunAllAsyncFn       func(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
	RunEphemeralAsyncFn func(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
	RunTestAsyncFn      func(ctx context.Context, hookID string, testID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
	RunTagAsyncFn       func(ctx context.Context, hookID string, testTag string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
}

func (c *MockAPITester) GetProject(ctx context.Context, hookID string) (ProjectMeta, error) {
	return c.GetProjectFn(ctx, hookID)
}

func (c *MockAPITester) GetEventResult(ctx context.Context, hookID string, eventID string) (TestResult, error) {
	return c.GetEventResultFn(ctx, hookID, eventID)
}

func (c *MockAPITester) GetTest(ctx context.Context, hookID string, testID string) (Test, error) {
	return c.GetTestFn(ctx, hookID, testID)
}

func (c *MockAPITester) GetProjects(ctx context.Context) ([]ProjectMeta, error) {
	return c.GetProjectsFn(ctx)
}

func (c *MockAPITester) GetHooks(ctx context.Context, projectID string) ([]Hook, error) {
	return c.GetHooksFn(ctx, projectID)
}

func (c *MockAPITester) RunAllAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error) {
	return c.RunAllAsyncFn(ctx, hookID, buildID, tunnel, test)
}

func (c *MockAPITester) RunEphemeralAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error) {
	return c.RunEphemeralAsyncFn(ctx, hookID, buildID, tunnel, test)
}

func (c *MockAPITester) RunTestAsync(ctx context.Context, hookID string, testID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error) {
	return c.RunTestAsyncFn(ctx, hookID, testID, buildID, tunnel, test)
}

func (c *MockAPITester) RunTagAsync(ctx context.Context, hookID string, testTag string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error) {
	return c.RunTestAsyncFn(ctx, hookID, testTag, buildID, tunnel, test)
}

func createTestDirs(t *testing.T) *fs.Dir {
	return fs.NewDir(t, "tests",
		fs.WithDir("tests",
			fs.WithDir("01_basic_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "yaml-unit-content", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "yaml-input-content", fs.WithMode(0644)),
			),
			fs.WithDir("02_extended_test", fs.WithMode(0755),
				fs.WithFile("unit.yml", "", fs.WithMode(0644)),
				fs.WithFile("input.yml", "", fs.WithMode(0644)),
			),
			fs.WithDir("03_no_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
			),
			fs.WithDir("04_no_test", fs.WithMode(0755),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
			),
			fs.WithDir("05_nested_tests", fs.WithMode(0755),
				fs.WithDir("06_nested_child01", fs.WithMode(0755),
					fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
					fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				),
				fs.WithDir("07_nested_child02", fs.WithMode(0755),
					fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
					fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				),
			),
			fs.WithDir("08_hybrid_tests", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				fs.WithDir("09_hybrid_tests", fs.WithMode(0755),
					fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
					fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				),
			),
		),
	)
}

func Test_findTests(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	type args struct {
		rootDir string
		match   []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "no filters",
			args: args{
				rootDir: dir.Path(),
				match:   []string{},
			},
			want: []string{
				"tests/01_basic_test",
				"tests/02_extended_test",
				"tests/05_nested_tests/06_nested_child01",
				"tests/05_nested_tests/07_nested_child02",
				"tests/08_hybrid_tests",
				"tests/08_hybrid_tests/09_hybrid_tests",
			},
		},
		{
			name: "single filter",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"01_basic_test",
				},
			},
			want: []string{"tests/01_basic_test"},
		},
		{
			name: "single filter - no match",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"dummy test filter",
				},
			},
		},
		{
			name: "multiple filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"01_basic_test",
					"02_extended_test",
				},
			},
			want: []string{
				"tests/01_basic_test",
				"tests/02_extended_test",
			},
		},
		{
			name: "multiple filters - no match",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"dummy filter",
					"second dummy filter",
				},
			},
		},
		{
			name: "single nested filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"05_nested_tests",
				},
			},
			want: []string{
				"tests/05_nested_tests/06_nested_child01",
				"tests/05_nested_tests/07_nested_child02",
			},
		},
		{
			name: "single nested hybrid filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"08_hybrid_tests",
				},
			},
			want: []string{
				"tests/08_hybrid_tests",
				"tests/08_hybrid_tests/09_hybrid_tests",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findTests(tt.args.rootDir, tt.args.match)
			if (err != nil) != tt.wantErr {
				t.Errorf("findTests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findTests() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newTestRequest(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	type args struct {
		testDir   string
		suiteName string
		testName  string
		tags      []string
	}
	tests := []struct {
		name    string
		args    args
		want    TestRequest
		wantErr bool
	}{
		{
			name: "Valid loading",
			args: args{
				tags:      []string{},
				testName:  "tests/01_basic_test",
				suiteName: "suiteName",
				testDir:   path.Join(dir.Path(), "tests/01_basic_test"),
			},
			wantErr: false,
			want: TestRequest{
				Name:  "suiteName - tests/01_basic_test",
				Tags:  []string{},
				Unit:  "yaml-unit-content",
				Input: "yaml-input-content",
			},
		},
		{
			name: "Buggy loading - missing input",
			args: args{
				tags:      []string{},
				testName:  "tests/03_no_test",
				suiteName: "suiteName",
				testDir:   path.Join(dir.Path(), "tests/03_no_test"),
			},
			wantErr: true,
		},
		{
			name: "Buggy loading - missing unit",
			args: args{
				tags:      []string{},
				testName:  "tests/04_no_test",
				suiteName: "suiteName",
				testDir:   path.Join(dir.Path(), "tests/04_no_test"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newTestRequest(tt.args.testDir, tt.args.suiteName, tt.args.testName, tt.args.tags, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("newTestRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newTestRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildTestName(t *testing.T) {
	type args struct {
		project ProjectMeta
		test    Test
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Basic Test",
			args: args{
				test: Test{
					Name: "defaultName",
				},
				project: ProjectMeta{
					Name: "projectName",
				},
			},
			want: "projectName - defaultName",
		},
		{
			name: "Only ProjectName",
			args: args{
				project: ProjectMeta{
					Name: "projectName",
				},
			},
			want: "projectName",
		},
		{
			name: "Only TestName",
			args: args{
				test: Test{
					Name: "defaultName",
				},
			},
			want: " - defaultName",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTestName(tt.args.project, tt.args.test); got != tt.want {
				t.Errorf("buildTestName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunner_newTestRequests(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	type args struct {
		s     Suite
		tests []string
	}
	tests := []struct {
		name string
		args args
		want []TestRequest
	}{
		{
			name: "Complete tests",
			args: args{
				s: Suite{
					Name: "suiteName",
				},
				tests: []string{
					"tests/01_basic_test",
				},
			},
			want: []TestRequest{
				{
					Name:  "suiteName - tests/01_basic_test",
					Unit:  "yaml-unit-content",
					Input: "yaml-input-content",
					Tags:  []string{},
				},
			},
		},
	}

	r := Runner{
		Project: Project{
			RootDir: dir.Path(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.newTestRequests(tt.args.s, tt.args.tests); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newTestRequests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunner_runLocalTests(t *testing.T) {
	pollWaitTime = 1 * time.Millisecond
	dir := createTestDirs(t)
	defer dir.Remove()

	mock := MockAPITester{
		GetProjectFn: func(context.Context, string) (ProjectMeta, error) {
			return ProjectMeta{
				ID:   "test",
				Name: "Testi",
			}, nil
		},
		RunEphemeralAsyncFn: func(context.Context, string, string, config.Tunnel, TestRequest) (AsyncResponse, error) {
			return AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			}, nil
		},
		GetEventResultFn: func(context.Context, string, string) (TestResult, error) {
			return TestResult{
				EventID:       "c4ca4238a0b923820dcc509a",
				FailuresCount: 0,
				Project: ProjectMeta{
					Name: "Test Project",
					ID:   "6244d915ca28694aab958bbe",
				},
				Test: Test{
					Name: "test_demo",
					ID:   "638788b12d29c47170999eee",
				},
				ExecutionTimeSeconds: 31,
				Async:                false,
				TimedOut:             false,
			}, nil
		},
	}

	r := Runner{
		Client: &mock,
		Project: Project{
			RootDir: dir.Path(),
		},
	}
	s := Suite{
		HookID:    "dummyHookId",
		Name:      "Basic Test",
		TestMatch: []string{"01_basic_test"},
		Tags:      []string{"canfail"},
	}
	c := make(chan TestResult)

	res := r.runLocalTests(context.Background(), s, c)
	assert.Equal(t, 1, res)

	results := <-c
	assert.Equal(t, TestResult{
		EventID:       "c4ca4238a0b923820dcc509a",
		FailuresCount: 0,
		Project: ProjectMeta{
			ID:   "6244d915ca28694aab958bbe",
			Name: "Test Project",
		},
		Test: Test{
			Name: "test_demo",
			ID:   "638788b12d29c47170999eee",
		},
		ExecutionTimeSeconds: 31,
	}, results)
}

func TestRunner_ResolveHookIDs(t *testing.T) {
	mock := MockAPITester{
		GetProjectsFn: func(context.Context) ([]ProjectMeta, error) {
			return []ProjectMeta{
				{
					ID:   "noHooks",
					Name: "Project NoHooks",
				},
				{
					ID:   "single",
					Name: "Project SingleHook",
				},
				{
					ID:   "multiple",
					Name: "Project MultipleHooks",
				},
				{
					ID:   "buggy",
					Name: "Project BuggyHooks",
				},
			}, nil
		},
		GetHooksFn: func(_ context.Context, projectID string) ([]Hook, error) {
			switch projectID {
			case "single":
				return []Hook{
					{
						Identifier: "uuid1",
						Name:       "name1",
					},
				}, nil
			case "multiple":
				return []Hook{
					{
						Identifier: "uuid1",
						Name:       "name1",
					},
					{
						Identifier: "uuid2",
						Name:       "name2",
					},
				}, nil
			}

			return []Hook{}, nil
		},
	}

	type fields struct {
		Project       Project
		Client        APITester
		Region        region.Region
		Reporters     []report.Reporter
		Async         bool
		TunnelService tunnel.Service
	}
	tests := []struct {
		name    string
		fields  fields
		want    Project
		wantErr string
	}{
		{
			name: "Project with no Hooks",
			fields: fields{
				Client: &mock,
				Project: Project{
					Suites: []Suite{
						{
							Name:        "Suite #1",
							ProjectName: "Project NoHooks",
						},
					},
				},
			},
			wantErr: msg.FailedToPrepareSuites,
		},
		{
			name: "Project with single Hooks",
			fields: fields{
				Client: &mock,
				Project: Project{
					Suites: []Suite{
						{
							Name:        "Suite #1",
							ProjectName: "Project SingleHook",
							ProjectID:   "single",
						},
					},
				},
			},
			want: Project{
				Suites: []Suite{
					{
						Name:        "Suite #1",
						ProjectName: "Project SingleHook",
						ProjectID:   "single",
						HookID:      "uuid1",
					},
				},
			},
		},
		{
			name: "Project with multiple Hooks",
			fields: fields{
				Client: &mock,
				Project: Project{
					Suites: []Suite{
						{
							Name:        "Suite #1",
							ProjectName: "Project MultipleHooks",
							ProjectID:   "multiple",
						},
					},
				},
			},
			want: Project{
				Suites: []Suite{
					{
						Name:        "Suite #1",
						ProjectName: "Project MultipleHooks",
						ProjectID:   "multiple",
						HookID:      "uuid1",
					},
				},
			},
		},
		{
			name: "Project with Buggy Hooks",
			fields: fields{
				Client: &mock,
				Project: Project{
					Suites: []Suite{
						{
							Name:        "Suite #1",
							ProjectName: "Project BuggyHooks",
						},
					},
				},
			},
			wantErr: msg.FailedToPrepareSuites,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				Project:       tt.fields.Project,
				Client:        tt.fields.Client,
				Region:        tt.fields.Region,
				Reporters:     tt.fields.Reporters,
				Async:         tt.fields.Async,
				TunnelService: tt.fields.TunnelService,
			}

			err := r.ResolveHookIDs(context.Background())
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr, "ResolveHookIDs(): got %v, want %v", err, tt.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("ResolveHookIDs(): got err: %v", err)
				return
			}

			if !reflect.DeepEqual(tt.want, tt.fields.Project) {
				t.Errorf("ResolveHookIDs(): got %v, want %v", tt.fields.Project, tt.want)
			}
		})
	}
}
