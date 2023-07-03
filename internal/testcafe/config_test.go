package testcafe

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func Test_appleDeviceRegex(t *testing.T) {
	tests := []struct {
		deviceName string
		want       bool
	}{
		{
			deviceName: "iPhone Simulator",
			want:       true,
		},
		{
			deviceName: "iphone simulator",
			want:       true,
		},
		{
			deviceName: "iPhone SE (2nd generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPhone 8 Simulator",
			want:       true,
		},
		{
			deviceName: "iPhone 8 Plus Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Pro (12.9 inch) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Pro (12.9 inch) (4th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Air Simulator",
			want:       true,
		},
		{
			deviceName: "iPad (8th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad mini (5th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "Android GoogleAPI Emulator",
			want:       false,
		},
		{
			deviceName: "Google Pixel 3a GoogleAPI Emulator",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.deviceName, func(t *testing.T) {
			got := appleDeviceRegex.MatchString(tt.deviceName)
			if got != tt.want {
				t.Errorf("appleDeviceRegex.MatchString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSuites(t *testing.T) {
	testCase := []struct {
		name      string
		config    *Project
		suiteName string
		expConfig Project
		expErr    string
	}{
		{
			name: "filtered suite exists in config",
			config: &Project{Suites: []Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite1",
			expConfig: Project{Suites: []Suite{
				{
					Name: "suite1",
				},
			}},
		},
		{
			name: "filtered suite does not exist in config",
			config: &Project{Suites: []Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite3",
			expConfig: Project{Suites: []Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			expErr: "no suite named 'suite3' found",
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			err := FilterSuites(tc.config, tc.suiteName)
			if err != nil {
				assert.Equal(t, tc.expErr, err.Error())
			}
			assert.True(t, reflect.DeepEqual(*tc.config, tc.expConfig))
		})
	}
}

func Test_shardSuites_withSplit(t *testing.T) {
	dir := fs.NewDir(t, "testcafe",
		fs.WithFile(".sauceignore", "", fs.WithMode(0644)),
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

	// Beginning state
	rootDir := dir.Path()
	origSuites := []Suite{
		{
			Name:  "Demo Suite",
			Src:   []string{"tests/**/*.js"},
			Shard: "spec",
		},
	}

	expectedSuites := []Suite{
		{
			Name:  "Demo Suite - tests/dir1/example1.tests.js",
			Src:   []string{"tests/dir1/example1.tests.js"},
			Shard: "spec",
		},
		{
			Name:  "Demo Suite - tests/dir2/example2.tests.js",
			Src:   []string{"tests/dir2/example2.tests.js"},
			Shard: "spec",
		},
		{
			Name:  "Demo Suite - tests/dir3/example3.tests.js",
			Src:   []string{"tests/dir3/example3.tests.js"},
			Shard: "spec",
		},
	}
	var err error
	var suites []Suite

	// Absolute path
	suites, err = shardSuites(rootDir, origSuites, 1, dir.Join(".sauceignore"))

	assert.Equal(t, err, nil)
	assert.Equal(t, expectedSuites, suites)

	// Relative path
	if err := os.Chdir(rootDir); err != nil {
		t.Errorf("Unexpected error %s", err)
	}
	suites, err = shardSuites(".", origSuites, 1, dir.Join(".sauceignore"))

	assert.Equal(t, err, nil)
	assert.Equal(t, expectedSuites, suites)
}

func Test_shardSuites_withoutSplit(t *testing.T) {
	dir := fs.NewDir(t, "testcafe",
		fs.WithFile(".sauceignore", "", fs.WithMode(0644)),
	)
	defer dir.Remove()
	origSuites := []Suite{
		{
			Name: "Demo Suite",
			Src:  []string{"tests/**/*.js"},
		},
	}
	var err error
	var suites []Suite

	// Absolute path
	suites, err = shardSuites("", origSuites, 1, dir.Join(".sauceignore"))

	assert.Equal(t, err, nil)
	assert.Equal(t, origSuites, suites)
}

func Test_shardSuites_withSplitNoMatch(t *testing.T) {
	dir := fs.NewDir(t, "testcafe",
		fs.WithFile(".sauceignore", "", fs.WithMode(0644)),
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

	// Beginning state
	rootDir := dir.Path()
	origSuites := []Suite{
		{
			Name:  "Demo Suite",
			Src:   []string{"dummy/**/*.js"},
			Shard: "spec",
		},
	}

	expectedSuites := make([]Suite, 0)
	var err error
	var suites []Suite

	// Absolute path
	suites, err = shardSuites(rootDir, origSuites, 1, dir.Join(".sauceignore"))

	assert.Equal(t, err, errors.New("suite 'Demo Suite' patterns have no matching files"))
	assert.Equal(t, expectedSuites, suites)

	// Relative path
	if err := os.Chdir(rootDir); err != nil {
		t.Errorf("Unexpected error %s", err)
	}
	suites, err = shardSuites(".", origSuites, 1, dir.Join(".sauceignore"))

	assert.Equal(t, err, errors.New("suite 'Demo Suite' patterns have no matching files"))
	assert.Equal(t, expectedSuites, suites)
}

func TestTestcafe_SortByHistory(t *testing.T) {
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

func TestTestcafe_FilterFailedTests(t *testing.T) {
	testcases := []struct {
		name      string
		suiteName string
		report    saucereport.SauceReport
		project   *Project
		expResult string
		expErr    error
	}{
		{
			name:      "it should set failed tests to specified suite",
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
			name:      "it should keep the original settings when suiteName doesn't exist in the project",
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
			name:      "it should keep the original settings when no failed test in SauceReport",
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
			assert.Equal(t, tc.expResult, tc.project.Suites[0].Filter.TestGrep)
		})
	}
}
