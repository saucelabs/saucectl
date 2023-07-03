package v1

import (
	"errors"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/saucereport"
	"github.com/stretchr/testify/assert"
)

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
			err := tc.config.FilterSuites(tc.suiteName)
			if err != nil {
				assert.Equal(t, tc.expErr, err.Error())
			}
			assert.True(t, reflect.DeepEqual(*tc.config, tc.expConfig))
		})
	}
}

func TestCypressV1_Validate(t *testing.T) {
	testCase := []struct {
		name    string
		project *Project
		expErr  bool
	}{
		{
			name: "empty cypress version",
			project: &Project{
				Cypress: Cypress{},
			},
			expErr: true,
		},
		{
			name: "empty invalid docker config",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
			},
			expErr: true,
		},
		{
			name: "invalid rootDir",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "./invalid_file",
			},
			expErr: true,
		},
		{
			name: "invalid region",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Sauce:   config.SauceConfig{Region: ""},
			},
			expErr: true,
		},
		{
			name: "empty suite",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Sauce:   config.SauceConfig{Region: "us-west-1"},
				Suites:  []Suite{},
			},
			expErr: true,
		},
		{
			name: "dup suite",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Sauce:   config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					Suite{
						Name:    "suite1",
						Browser: "chrome",
						Config: SuiteConfig{
							SpecPattern: []string{"a"},
						},
					},
					Suite{
						Name:    "suite1",
						Browser: "chrome",
						Config: SuiteConfig{
							SpecPattern: []string{"a"},
						},
					},
				},
			},
			expErr: true,
		},
		{
			name: "symbol in suite name",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Sauce:   config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					Suite{
						Name:    string('\u212A'),
						Browser: "chrome",
						Config: SuiteConfig{
							SpecPattern: []string{"a"},
						},
					},
				},
			},
			expErr: true,
		},
		{
			name: "empty browser name",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Sauce:   config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					Suite{
						Name: "suite 1",
						Config: SuiteConfig{
							SpecPattern: []string{"a"},
						},
					},
					Suite{
						Name: "suite 2",
						Config: SuiteConfig{
							SpecPattern: []string{"a"},
						},
					},
				},
			},
			expErr: true,
		},
		{
			name: "empty testingType",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Sauce:   config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					Suite{
						Name:    "suite 1",
						Browser: "chrome",
						Config: SuiteConfig{
							TestingType: "e2e",
							SpecPattern: []string{"a"},
						},
					},
					Suite{
						Name:    "suite 2",
						Browser: "chrome",
						Config: SuiteConfig{
							TestingType: "e2e",
							SpecPattern: []string{"a"},
						},
					},
				},
			},
			expErr: true,
		},

		{
			name: "empty specPattern",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Sauce:   config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					Suite{
						Name:    "suite 1",
						Browser: "chrome",
						Config: SuiteConfig{
							TestingType: "e2e",
							SpecPattern: []string{"a"},
						},
					},
					Suite{
						Name:    "suite 2",
						Browser: "chrome",
						Config: SuiteConfig{
							TestingType: "e2e",
						},
					},
				},
			},
			expErr: true,
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.project.Validate()
			if err != nil {
				assert.True(t, tc.expErr)
			}
		})
	}
}

func TestCyress_CleanPackages(t *testing.T) {
	testCase := []struct {
		name      string
		project   Project
		expResult map[string]string
	}{
		{
			name: "clean cypress package in npm packages",
			project: Project{Npm: config.Npm{
				Packages: map[string]string{
					"cypress": "10.1.0",
				},
			}},
			expResult: map[string]string{},
		},
		{
			name: "no need to clean npm packages",
			project: Project{Npm: config.Npm{
				Packages: map[string]string{
					"lodash": "*",
				},
			}},
			expResult: map[string]string{
				"lodash": "*",
			},
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			tc.project.CleanPackages()
			assert.Equal(t, tc.expResult, tc.project.Npm.Packages)
		})
	}
}

func TestCypressV1_FilterFailedTests(t *testing.T) {
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
			expResult: "failed test1;failed test2",
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
			assert.Equal(t, tc.expResult, tc.project.Suites[0].Config.Env["grep"])
		})
	}
}
