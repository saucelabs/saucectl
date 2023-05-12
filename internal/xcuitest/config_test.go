package xcuitest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/insights"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestValidate(t *testing.T) {
	dir := fs.NewDir(t, "xcuitest-config",
		fs.WithFile("test.ipa", "", fs.WithMode(0655)),
		fs.WithFile("testApp.ipa", "", fs.WithMode(0655)),
		fs.WithDir("test.app", fs.WithMode(0755)),
		fs.WithDir("testApp.app", fs.WithMode(0755)))
	defer dir.Remove()
	appF := filepath.Join(dir.Path(), "test.ipa")
	testAppF := filepath.Join(dir.Path(), "testApp.ipa")
	appD := filepath.Join(dir.Path(), "test.app")
	testAppD := filepath.Join(dir.Path(), "testApp.app")

	testCases := []struct {
		name        string
		p           *Project
		expectedErr error
	}{
		{
			name: "validating throws error on empty app",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
			},
			expectedErr: errors.New("missing path to app .ipa"),
		},
		{
			name: "validating passing with .ipa",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
				Suites: []Suite{
					{
						Name: "iphone",
						Devices: []config.Device{
							{Name: "iPhone.*"},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "validating passing with .app",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appD,
					TestApp: testAppD,
				},
				Suites: []Suite{
					{
						Name: "iphone",
						Devices: []config.Device{
							{Name: "iPhone.*"},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "validating error with app other than .ipa / .app",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     "/path/to/app.zip",
					TestApp: testAppD,
				},
			},
			expectedErr: errors.New("invalid application file: /path/to/app.zip, make sure extension is one of the following: .ipa, .app"),
		},
		{
			name: "validating error with test app other than .ipa / .app",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: "/path/to/app.zip",
				},
			},
			expectedErr: errors.New("invalid test application file: /path/to/app.zip, make sure extension is one of the following: .ipa, .app"),
		},
		{
			name: "validating throws error on empty testApp",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: "",
				},
			},
			expectedErr: errors.New("missing path to test app .ipa"),
		},
		{
			name: "validating throws error on not test app .ipa",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: "/path/to/bundle/tests",
				},
			},
			expectedErr: errors.New("invalid test application file: /path/to/bundle/tests, make sure extension is one of the following: .ipa, .app"),
		},
		{
			name: "validating throws error on missing suites",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
			},
			expectedErr: errors.New("no suites defined"),
		},
		{
			name: "validating throws error on missing devices",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
				Suites: []Suite{
					{
						Name:    "no devices",
						Devices: []config.Device{},
					},
				},
			},
			expectedErr: errors.New("missing devices configuration for suite: no devices"),
		},
		{
			name: "validating throws error on missing device name",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
				Suites: []Suite{
					{
						Name: "no device name",
						Devices: []config.Device{
							{
								Name: "",
							},
						},
					},
				},
			},
			expectedErr: errors.New("missing device name or ID for suite: no device name. Devices index: 0"),
		},
		{
			name: "validating throws error on unsupported device type",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
				Suites: []Suite{
					{
						Name: "unsupported device type",
						Devices: []config.Device{
							{
								Name:         "iPhone 11",
								PlatformName: "iOS",
								Options: config.DeviceOptions{
									DeviceType: "some",
								},
							},
						},
					},
				},
			},
			expectedErr: errors.New("deviceType: some is unsupported for suite: unsupported device type. Devices index: 0. Supported device types: ANY,PHONE,TABLET"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(*tc.p)
			if tc.expectedErr == nil && err != nil {
				t.Errorf("want: %v, got: %v", tc.expectedErr, err)
			}
			if tc.expectedErr != nil && tc.expectedErr.Error() != err.Error() {
				t.Errorf("want: %v, got: %v", tc.expectedErr, err)
			}
		})
	}
}

func TestFromFile(t *testing.T) {
	dir := fs.NewDir(t, "xcuitest-cfg",
		fs.WithFile("config.yml", `apiVersion: v1alpha
kind: xcuitest
xcuitest:
  app: "./tests/apps/xcuitest/SauceLabs.Mobile.Sample.XCUITest.App.ipa"
  testApp: "./tests/apps/xcuitest/SwagLabsMobileAppUITests-Runner.ipa"
suites:
  - name: "saucy barista"
    devices:
      - name: "iPhone XR"
        platformVersion: "14.3"
    testOptions:
      class: ["SwagLabsMobileAppUITests.LoginTests/testSuccessfulLogin", "SwagLabsMobileAppUITests.LoginTests"]
`, fs.WithMode(0655)))
	defer dir.Remove()

	cfg, err := FromFile(filepath.Join(dir.Path(), "config.yml"))
	if err != nil {
		t.Errorf("expected error: %v, got: %v", nil, err)
	}
	expected := Project{
		Xcuitest: Xcuitest{
			App:     "./tests/apps/xcuitest/SauceLabs.Mobile.Sample.XCUITest.App.ipa",
			TestApp: "./tests/apps/xcuitest/SwagLabsMobileAppUITests-Runner.ipa",
		},
		Suites: []Suite{
			{
				Name: "saucy barista",
				Devices: []config.Device{
					{
						Name:            "iPhone XR",
						PlatformVersion: "14.3",
					},
				},
				TestOptions: TestOptions{
					Class: []string{
						"SwagLabsMobileAppUITests.LoginTests/testSuccessfulLogin",
						"SwagLabsMobileAppUITests.LoginTests",
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(cfg.Xcuitest, expected.Xcuitest) {
		t.Errorf("expected: %v, got: %v", expected, cfg)
	}
	if !reflect.DeepEqual(cfg.Suites, expected.Suites) {
		t.Errorf("expected: %v, got: %v", expected, cfg)
	}
}

func TestSetDefaults_Platform(t *testing.T) {
	type args struct {
		Device config.Device
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no platform specified",
			args: args{Device: config.Device{}},
			want: "iOS",
		},
		{
			name: "wrong platform specified",
			args: args{Device: config.Device{PlatformName: "myOS"}},
			want: "iOS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Project{Suites: []Suite{{
				Devices: []config.Device{tt.args.Device},
			}}}

			SetDefaults(&p)

			got := p.Suites[0].Devices[0].PlatformName
			if got != tt.want {
				t.Errorf("SetDefaults() got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestSetDefaults_DeviceType(t *testing.T) {
	type args struct {
		Device config.Device
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "device type is always uppercase",
			args: args{Device: config.Device{Options: config.DeviceOptions{DeviceType: "phone"}}},
			want: "PHONE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Project{Suites: []Suite{{
				Devices: []config.Device{tt.args.Device},
			}}}

			SetDefaults(&p)

			got := p.Suites[0].Devices[0].Options.DeviceType
			if got != tt.want {
				t.Errorf("SetDefaults() got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestSetDefaults_TestApp(t *testing.T) {
	testCase := []struct {
		name      string
		project   Project
		expResult string
	}{
		{
			name: "Set TestApp on suite level",
			project: Project{
				Xcuitest: Xcuitest{
					TestApp: "test-app",
				},
				Suites: []Suite{
					{
						TestApp: "suite-test-app",
					},
				},
			},
			expResult: "suite-test-app",
		},
		{
			name: "Set empty TestApp on suite level",
			project: Project{
				Xcuitest: Xcuitest{
					TestApp: "test-app",
				},
				Suites: []Suite{
					{},
				},
			},
			expResult: "test-app",
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			SetDefaults(&tc.project)
			assert.Equal(t, tc.expResult, tc.project.Suites[0].TestApp)
		})
	}
}

func TestXCUITest_SortByHistory(t *testing.T) {
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

func TestXCUITest_ShardSuites(t *testing.T) {
	testCases := []struct {
		name          string
		project       Project
		content       string
		configEnabled bool
		expSuites     []Suite
		expErr        bool
	}{
		{
			name: "should keep original test options when sharding is disabled",
			project: Project{
				Suites: []Suite{
					{
						Name: "no shard",
						TestOptions: TestOptions{
							Class: []string{"no update"},
						},
					},
				},
			},
			expSuites: []Suite{
				{
					Name: "no shard",
					TestOptions: TestOptions{
						Class: []string{"no update"},
					},
				},
			},
		},
		{
			name: "should shard tests by ccy when sharding is enabled",
			project: Project{
				Sauce: config.SauceConfig{
					Concurrency: 2,
				},
				Suites: []Suite{
					{
						Name:  "sharding test",
						Shard: "concurrency",
					},
				},
			},
			content:       "test1\ntest2\n",
			configEnabled: true,
			expSuites: []Suite{
				{
					Name: "sharding test - 1/2",
					TestOptions: TestOptions{
						Class: []string{"test1"},
					},
				},
				{
					Name: "sharding test - 2/2",
					TestOptions: TestOptions{
						Class: []string{"test2"},
					},
				},
			},
		},
		{
			name: "should keep original test options when sharding w/o testClassesFile",
			project: Project{
				Sauce: config.SauceConfig{
					Concurrency: 2,
				},
				Suites: []Suite{
					{
						Name:  "sharding test",
						Shard: "concurrency",
						TestOptions: TestOptions{
							Class: []string{"test1"},
						},
					},
				},
			},
			configEnabled: false,
			expSuites: []Suite{
				{
					Name: "sharding test",
					TestOptions: TestOptions{
						Class: []string{"test1"},
					},
				},
			},
			expErr: true,
		},
		{
			name: "should keep original test options when sharding w/ empty testClassesFile",
			project: Project{
				Sauce: config.SauceConfig{
					Concurrency: 2,
				},
				Suites: []Suite{
					{
						Name:  "sharding test",
						Shard: "concurrency",
						TestOptions: TestOptions{
							Class: []string{"test1"},
						},
					},
				},
			},
			configEnabled: true,
			content:       "",
			expSuites: []Suite{
				{
					Name: "sharding test",
					TestOptions: TestOptions{
						Class: []string{"test1"},
					},
				},
			},
			expErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var testClassesFile string
			if tc.configEnabled {
				testClassesFile = createShardConfig(tc.content)
				tc.project.Suites[0].TestClassesFile = testClassesFile
			}
			err := ShardSuites(&tc.project)
			if err != nil {
				assert.True(t, tc.expErr)
			}
			for i, s := range tc.project.Suites {
				assert.True(t, cmp.Equal(s.TestOptions, tc.expSuites[i].TestOptions))
				assert.True(t, cmp.Equal(s.Name, tc.expSuites[i].Name))
			}

			t.Cleanup(func() {
				if testClassesFile != "" {
					os.RemoveAll(testClassesFile)
				}
			})
		})
	}
}

func createShardConfig(content string) string {
	tmpDir, _ := os.MkdirTemp("", "shard")
	file := filepath.Join(tmpDir, "tests.txt")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		fmt.Println(err)
		return ""
	}
	return file
}
