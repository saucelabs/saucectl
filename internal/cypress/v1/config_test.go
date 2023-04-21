package v1

import (
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
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
				Docker:  config.Docker{FileTransfer: "test"},
			},
			expErr: true,
		},
		{
			name: "invalid rootDir",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "./invalid_file",
				Docker:  config.Docker{FileTransfer: "mount"},
			},
			expErr: true,
		},
		{
			name: "invalid region",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Docker:  config.Docker{FileTransfer: "mount"},
				Sauce:   config.SauceConfig{Region: ""},
			},
			expErr: true,
		},
		{
			name: "empty suite",
			project: &Project{
				Cypress: Cypress{Version: "v1.1.1"},
				RootDir: "../",
				Docker:  config.Docker{FileTransfer: "mount"},
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
				Docker:  config.Docker{FileTransfer: "mount"},
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
				Docker:  config.Docker{FileTransfer: "mount"},
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
				Docker:  config.Docker{FileTransfer: "mount"},
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
				Docker:  config.Docker{FileTransfer: "mount"},
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
				Docker:  config.Docker{FileTransfer: "mount"},
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

func TestCypress_SetTestGrep(t *testing.T) {
	testCases := []struct {
		name       string
		project    Project
		suiteIndex int
		tests      []string
		expResult  map[string]string
	}{
		{
			name: "set TestGrep for the first suite in project",
			project: Project{
				Suites: []Suite{
					{
						Config: SuiteConfig{
							Env: map[string]string{},
						},
					},
				},
			},
			suiteIndex: 0,
			tests:      []string{"failed test1", "failed test2"},
			expResult: map[string]string{
				"grep": "failed test1;failed test2",
			},
		},
		{
			name: "set TestGrep for the first suite in project w/ empty Env",
			project: Project{
				Suites: []Suite{
					{},
				},
			},
			suiteIndex: 0,
			tests:      []string{"failed test1", "failed test2"},
			expResult: map[string]string{
				"grep": "failed test1;failed test2",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.project.SetTestGrep(tc.suiteIndex, tc.tests)
			assert.Equal(t, tc.expResult, tc.project.Suites[tc.suiteIndex].Config.Env)
		})
	}
}
