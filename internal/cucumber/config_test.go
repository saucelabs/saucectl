package cucumber

import (
	"errors"
	"testing"

	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/stretchr/testify/assert"
)

func TestCucumber_FilterFailedTests(t *testing.T) {
	testcases := []struct {
		name      string
		suiteName string
		report    saucereport.SauceReport
		project   *Project
		expResult []string
		expErr    error
	}{
		{
			name:      "suite exists and failed tests exist",
			suiteName: "my suite",
			report: saucereport.SauceReport{
				Status: saucereport.StatusFailed,
				Suites: []saucereport.Suite{
					{
						Name:   "my_test.feature",
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
					{
						Name:   "my_test2.feature",
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
						Options: Options{
							Paths: []string{
								"my_test.feature",
								"my_test2.feature",
								"my_test3.feature",
							},
						},
					},
				},
			},
			expResult: []string{"my_test.feature", "my_test2.feature"},
			expErr:    nil,
		},
		{
			name:      "suite desn't exist",
			suiteName: "my suite2",
			report: saucereport.SauceReport{
				Status: saucereport.StatusFailed,
				Suites: []saucereport.Suite{
					{
						Name:   "my_test.feature",
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
						Options: Options{
							Paths: []string{
								"my_test.feature",
								"my_test2.feature",
								"my_test3.feature",
							},
						},
					},
				},
			},
			expResult: []string{
				"my_test.feature",
				"my_test2.feature",
				"my_test3.feature",
			},
			expErr: errors.New("suite(my suite2) not found"),
		},
		{
			name:      "no failed tests",
			suiteName: "my suite",
			report: saucereport.SauceReport{
				Status: saucereport.StatusPassed,
				Suites: []saucereport.Suite{
					{
						Name:   "my_test.feature",
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
						Options: Options{
							Paths: []string{
								"my_test.feature",
								"my_test2.feature",
								"my_test3.feature",
							},
						},
					},
				},
			},
			expResult: []string{
				"my_test.feature",
				"my_test2.feature",
				"my_test3.feature",
			},
			expErr: nil,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.project.FilterFailedTests(tc.suiteName, tc.report)
			assert.Equal(t, tc.expErr, err)
			assert.Equal(t, tc.expResult, tc.project.Suites[0].Options.Paths)
		})
	}
}
