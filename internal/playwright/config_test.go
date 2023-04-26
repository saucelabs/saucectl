package playwright

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/saucereport"
)

func Test_shardSuites(t *testing.T) {
	type args struct {
		suites []Suite
	}
	tests := []struct {
		name string
		args args
		want []Suite
	}{
		{
			name: "shard into three",
			args: args{[]Suite{{Name: "Test", NumShards: 3}}},
			want: []Suite{
				{Name: "Test (shard 1/3)", NumShards: 3, Params: SuiteConfig{Shard: "1/3"}},
				{Name: "Test (shard 2/3)", NumShards: 3, Params: SuiteConfig{Shard: "2/3"}},
				{Name: "Test (shard 3/3)", NumShards: 3, Params: SuiteConfig{Shard: "3/3"}},
			},
		},
		{
			name: "shard some",
			args: args{[]Suite{
				{Name: "Test", NumShards: 3},
				{Name: "Unsharded"},
			}},
			want: []Suite{
				{Name: "Test (shard 1/3)", NumShards: 3, Params: SuiteConfig{Shard: "1/3"}},
				{Name: "Test (shard 2/3)", NumShards: 3, Params: SuiteConfig{Shard: "2/3"}},
				{Name: "Test (shard 3/3)", NumShards: 3, Params: SuiteConfig{Shard: "3/3"}},
				{Name: "Unsharded"},
			},
		},
		{
			name: "shard nothing",
			args: args{[]Suite{
				{Name: "Test"},
				{Name: "Test", NumShards: 1},
			}},
			want: []Suite{
				{Name: "Test"},
				{Name: "Test", NumShards: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shardSuitesByNumShards(tt.args.suites); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shardSuites() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShardSuites(t *testing.T) {
	dir := fs.NewDir(t, "testcafe",
		fs.WithDir("tests",
			fs.WithMode(0755),
			fs.WithDir("dir1",
				fs.WithMode(0755),
				fs.WithFile("example1.tests.js", "", fs.WithMode(0644)),
			),
			fs.WithDir("dir2",
				fs.WithMode(0755),
				fs.WithFile("example2.tests.js", "", fs.WithMode(0644)),
			),
			fs.WithDir("dir3",
				fs.WithMode(0755),
				fs.WithFile("example3.tests.js", "", fs.WithMode(0644)),
			),
		),
	)
	defer dir.Remove()

	testCases := []struct {
		name           string
		p              *Project
		wantErr        bool
		expectedErrMsg string
		expectedSuites []Suite
	}{
		{
			name: "numShards and shard can't be used at the same time",
			p: &Project{Suites: []Suite{
				{
					Name:      "suite #1",
					NumShards: 2,
					Shard:     "spec",
				},
			}},
			wantErr:        true,
			expectedErrMsg: "suite name: suite #1 numShards and shard can't be used at the same time",
		},
		{
			name: "split by spec",
			p: &Project{
				RootDir: dir.Path(),
				Suites: []Suite{
					{
						Name:      "suite #1",
						Shard:     "spec",
						TestMatch: []string{".*.js"},
					},
				}},
			wantErr:        false,
			expectedErrMsg: "",
			expectedSuites: []Suite{
				{
					Name:      "suite #1 - tests/dir1/example1.tests.js",
					TestMatch: []string{"tests/dir1/example1.tests.js"},
					Shard:     "spec",
				},
				{
					Name:      "suite #1 - tests/dir2/example2.tests.js",
					TestMatch: []string{"tests/dir2/example2.tests.js"},
					Shard:     "spec",
				},
				{
					Name:      "suite #1 - tests/dir3/example3.tests.js",
					TestMatch: []string{"tests/dir3/example3.tests.js"},
					Shard:     "spec",
				},
			},
		},
		{
			name: "split by spec - no match",
			p: &Project{
				RootDir: dir.Path(),
				Suites: []Suite{
					{
						Name:      "suite #1",
						Shard:     "spec",
						TestMatch: []string{"failing.*.js"},
					},
				}},
			wantErr:        true,
			expectedErrMsg: "suite 'suite #1' patterns have no matching files",
			expectedSuites: []Suite{
				{
					Name:      "suite #1",
					TestMatch: []string{"tests/dir1/example1.tests.js"},
					Shard:     "spec",
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			err := ShardSuites(tt.p)
			if tt.wantErr {
				if err.Error() != tt.expectedErrMsg {
					t.Errorf("ShardSuites() = %v, want %v", err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.Equal(t, tt.expectedSuites, tt.p.Suites)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name    string
		p       Project
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing version",
			p:       Project{Playwright: Playwright{Version: "v"}},
			wantErr: true,
			errMsg:  "missing framework version. Check available versions here: https://docs.saucelabs.com/dev/cli/saucectl/#supported-frameworks-and-browsers",
		},
		{
			name: "unable to locate the rootDir folder",
			p: Project{
				Playwright: Playwright{Version: "v1.1.1"}, RootDir: "/test",
			},
			wantErr: true,
			errMsg:  "unable to locate the rootDir folder /test",
		},
		{
			name: "not supported browser",
			p: Project{
				Playwright: Playwright{Version: "v1.1.1"},
				Suites: []Suite{
					{Params: SuiteConfig{BrowserName: "ie"}},
				}},
			wantErr: true,
			errMsg:  "browserName: ie is not supported. List of supported browsers: chromium, firefox, webkit, chrome",
		},
		{
			name: "empty region",
			p: Project{
				Sauce:      config.SauceConfig{Region: ""},
				Playwright: Playwright{Version: "v1.1.1"},
				Suites: []Suite{
					{Name: "suite #1", NumShards: 2, Params: SuiteConfig{BrowserName: "chromium"}},
				}},
			wantErr: true,
			errMsg:  "no sauce region set",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				err := Validate(&tt.p)
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestPlaywright_SortByHistory(t *testing.T) {
	testCases := []struct {
		name    string
		suites  []Suite
		history insights.JobHistory
		expRes  []Suite
	}{
		{
			name: "sort suites by job history",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
				Suite{Name: "suite 3"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
				Suite{Name: "suite 3"},
			},
		},
		{
			name: "suites is the subset of job history",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
			},
		},
		{
			name: "job history is the subset of suites",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
				Suite{Name: "suite 3"},
				Suite{Name: "suite 4"},
				Suite{Name: "suite 5"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
				Suite{Name: "suite 3"},
				Suite{Name: "suite 4"},
				Suite{Name: "suite 5"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SortByHistory(tc.suites, tc.history)
			for i := 0; i < len(result); i++ {
				assert.Equal(t, tc.expRes[i].Name, result[i].Name)
			}
		})
	}
}

func TestPlaywright_FilterFailedTests(t *testing.T) {
	testcases := []struct {
		name      string
		suiteName string
		report    saucereport.SauceReport
		project   *Project
		expResult string
		expErr    error
	}{
		{
			name:      "suite exists and failed tests exist",
			suiteName: "my suite",
			report: saucereport.SauceReport{
				Status: saucereport.StatusFailed,
				Suites: []saucereport.Suite{
					{
						Name:   "my suite",
						Status: saucereport.StatusFailed,
						Tests: []saucereport.Test{
							{
								Status: saucereport.StatusFailed,
								Name:   "failed test1",
							},
							{
								Status: saucereport.StatusFailed,
								Name:   "failed test2",
							},
						},
					},
				},
			},
			project: &Project{
				Suites: []Suite{
					{
						Name: "my suite",
					},
				},
			},
			expResult: "failed test1|failed test2",
			expErr:    nil,
		},
		{
			name:      "suite doesn't exist",
			suiteName: "my suite2",
			report: saucereport.SauceReport{
				Status: saucereport.StatusFailed,
				Suites: []saucereport.Suite{
					{
						Name:   "my suite",
						Status: saucereport.StatusFailed,
						Tests: []saucereport.Test{
							{
								Status: saucereport.StatusFailed,
								Name:   "failed test1",
							},
							{
								Status: saucereport.StatusFailed,
								Name:   "failed test2",
							},
						},
					},
				},
			},
			project: &Project{
				Suites: []Suite{
					{
						Name: "my suite",
					},
				},
			},
			expResult: "",
			expErr:    errors.New("suite(my suite2) not found"),
		},
		{
			name:      "no failed tests",
			suiteName: "my suite",
			report: saucereport.SauceReport{
				Status: saucereport.StatusPassed,
				Suites: []saucereport.Suite{
					{
						Name:   "my suite",
						Status: saucereport.StatusPassed,
						Tests: []saucereport.Test{
							{
								Status: saucereport.StatusPassed,
								Name:   "passed test1",
							},
							{
								Status: saucereport.StatusSkipped,
								Name:   "skipped test2",
							},
						},
					},
				},
			},
			project: &Project{
				Suites: []Suite{
					{
						Name: "my suite",
					},
				},
			},
			expResult: "",
			expErr:    nil,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.project.FilterFailedTests(tc.suiteName, tc.report)
			assert.Equal(t, tc.expErr, err)
			assert.Equal(t, tc.expResult, tc.project.Suites[0].Params.Grep)
		})
	}
}
