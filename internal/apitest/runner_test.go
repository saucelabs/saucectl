package apitest

import (
	"github.com/saucelabs/saucectl/internal/apitesting"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"testing"
)

func createTestDirs(t *testing.T) *fs.Dir {
	return fs.NewDir(t, "tests",
		fs.WithDir("tests",
			fs.WithDir("01_basic_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "yaml-unit-content", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "yaml-input-content", fs.WithMode(0644)),
			),
			fs.WithDir("02_extended_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
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

func Test_loadTest(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	type args struct {
		unitPath  string
		inputPath string
		suiteName string
		testName  string
		tags      []string
	}
	tests := []struct {
		name    string
		args    args
		want    apitesting.TestRequest
		wantErr bool
	}{
		{
			name: "Valid loading",
			args: args{
				tags:      []string{},
				testName:  "tests/01_basic_test",
				suiteName: "suiteName",
				inputPath: path.Join(dir.Path(), "tests/01_basic_test/input.yaml"),
				unitPath:  path.Join(dir.Path(), "tests/01_basic_test/unit.yaml"),
			},
			wantErr: false,
			want: apitesting.TestRequest{
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
				inputPath: path.Join(dir.Path(), "tests/03_no_test/input.yaml"),
				unitPath:  path.Join(dir.Path(), "tests/03_no_test/unit.yaml"),
			},
			wantErr: true,
		},
		{
			name: "Buggy loading - missing unit",
			args: args{
				tags:      []string{},
				testName:  "tests/04_no_test",
				suiteName: "suiteName",
				inputPath: path.Join(dir.Path(), "tests/04_no_test/input.yaml"),
				unitPath:  path.Join(dir.Path(), "tests/04_no_test/unit.yaml"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadTest(tt.args.unitPath, tt.args.inputPath, tt.args.suiteName, tt.args.testName, tt.args.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadTest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadTest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildTestName(t *testing.T) {
	type args struct {
		project apitesting.Project
		test    apitesting.Test
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Basic Test",
			args: args{
				test: apitesting.Test{
					Name: "defaultName",
				},
				project: apitesting.Project{
					Name: "projectName",
				},
			},
			want: "projectName - defaultName",
		},
		{
			name: "Only ProjectName",
			args: args{
				project: apitesting.Project{
					Name: "projectName",
				},
			},
			want: "projectName",
		},
		{
			name: "Only TestName",
			args: args{
				test: apitesting.Test{
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

func TestRunner_loadTests(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	type args struct {
		s     Suite
		tests []string
	}
	tests := []struct {
		name string
		args args
		want []apitesting.TestRequest
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
			want: []apitesting.TestRequest{
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
			if got := r.loadTests(tt.args.s, tt.args.tests); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadTests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunner_runLocalTests(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/tests/_exec":
			_, _ = w.Write([]byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`))
		case "/api-testing/rest/v4/dummyHookId/insights/events/c4ca4238a0b923820dcc509a":
			completeStatusResp := []byte(`{"_id":"c4ca4238a0b923820dcc509a","events":[],"tags":["canfail"],"criticalFailures":[],"httpFailures":[],"facts":{},"date":1670258196613,"test":{"name":"test_demo","id":"638788b12d29c47170999eee"},"failuresCount":0,"warningsCount":0,"compressed":false,"run":{"name":"","id":""},"company":{"name":"","id":"7fb25570b4064716b9b6daae1a997bba"},"project":{"name":"Test Project","id":"6244d915ca28694aab958bbe"},"temp":false,"expireAt":"2023-06-06T04:37:07Z","executionTimeSeconds":31,"taskId":"ad24fdd6-8e47-401c-81ce-866553194bdd","agent":"wstestjs","mode":"ondemand","buildId":"Test","clientname":"","initiator":{"name":"Incitator","id":"de8691a22ff343f08aa6fb63e485fe0d","teamid":"0205cb60678a4372193bac4052c048be"}}`)
			_, _ = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	client := ts.Client()

	r := Runner{
		Client: apitesting.Client{
			HTTPClient: client,
			URL:        ts.URL,
		},
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
	c := make(chan []apitesting.TestResult)

	res := r.runLocalTests(s, c)
	assert.Equal(t, 1, res)

	results := <-c
	assert.Equal(t, []apitesting.TestResult{
		{
			EventID:       "c4ca4238a0b923820dcc509a",
			FailuresCount: 0,
			Project: apitesting.Project{
				ID:   "6244d915ca28694aab958bbe",
				Name: "Test Project",
			},
			Test: apitesting.Test{
				Name: "test_demo",
				ID:   "638788b12d29c47170999eee",
			},
			ExecutionTimeSeconds: 31,
		},
	}, results)
}

func TestRunner_ResolveHookIDs(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.URL.Path {
		case "/api-testing/api/project":
			completeStatusResp := []byte(`[{"id":"noHooks","name":"Project NoHooks"},{"id":"single","name":"Project SingleHook"},{"id":"multiple","name":"Project MultipleHooks"},{"id":"buggy","name":"Project BuggyHooks"}]`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/api/project/noHooks/hook":
			completeStatusResp := []byte(`[]`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/api/project/single/hook":
			completeStatusResp := []byte(`[{"id":"hook1","identifier":"uuid1","name":"name1","description":"description1"}]`)
			_, err = w.Write(completeStatusResp)
		case "/api-testing/api/project/multiple/hook":
			completeStatusResp := []byte(`[{"id":"hook1","identifier":"uuid1","name":"name1","description":"description1"},{"id":"hook2","identifier":"uuid2","name":"name2","description":"description2"}]`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()

	type fields struct {
		Project       Project
		Client        apitesting.Client
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
				Client: apitesting.Client{
					HTTPClient: ts.Client(),
					URL:        ts.URL,
				},
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
				Client: apitesting.Client{
					HTTPClient: ts.Client(),
					URL:        ts.URL,
				},
				Project: Project{
					Suites: []Suite{
						{
							Name:        "Suite #1",
							ProjectName: "Project SingleHook",
						},
					},
				},
			},
			want: Project{
				Suites: []Suite{
					{
						Name:        "Suite #1",
						ProjectName: "Project SingleHook",
						HookID:      "uuid1",
					},
				},
			},
		},
		{
			name: "Project with multiple Hooks",
			fields: fields{
				Client: apitesting.Client{
					HTTPClient: ts.Client(),
					URL:        ts.URL,
				},
				Project: Project{
					Suites: []Suite{
						{
							Name:        "Suite #1",
							ProjectName: "Project MultipleHooks",
						},
					},
				},
			},
			want: Project{
				Suites: []Suite{
					{
						Name:        "Suite #1",
						ProjectName: "Project MultipleHooks",
						HookID:      "uuid1",
					},
				},
			},
		},
		{
			name: "Project with Buggy Hooks",
			fields: fields{
				Client: apitesting.Client{
					HTTPClient: ts.Client(),
					URL:        ts.URL,
				},
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

			err := r.ResolveHookIDs()
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
