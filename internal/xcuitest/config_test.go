package xcuitest

import (
	"errors"
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

func TestTestOptions_ToMap(t *testing.T) {
	opts := TestOptions{
		Class:                             []string{},
		NotClass:                          []string{},
		TestLanguage:                      "",
		TestRegion:                        "",
		TestTimeoutsEnabled:               "",
		MaximumTestExecutionTimeAllowance: 20,
		DefaultTestExecutionTimeAllowance: 0,
		StatusBarOverrideTime:             "",
	}
	wantLength := 8

	m := opts.ToMap()

	if len(m) != wantLength {
		t.Errorf("Length of converted TestOptions should match original, got (%v) want (%v)", len(m), wantLength)
	}

	v := reflect.ValueOf(m["maximumTestExecutionTimeAllowance"])
	vtype := v.Type()
	if vtype.Kind() != reflect.String {
		t.Errorf("ints should be converted to strings when mapping, got (%v) want (%v)", vtype, reflect.String)
	}

	if v := m["defaultTestExecutionTimeAllowance"]; v != "" {
		t.Errorf("0 values should be cast to empty strings, got (%v)", v)
	}
}

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
				Suites: []Suite{
					{
						Name: "suite with missing app",
						Devices: []config.Device{
							{Name: "iPhone.*"},
						},
					},
				},
			},
			expectedErr: errors.New("missing path to app .ipa"),
		},
		{
			name: "validating passing with .ipa",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					{
						Name:    "iphone",
						App:     appF,
						TestApp: testAppF,
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
				Suites: []Suite{
					{
						Name:    "iphone",
						App:     appD,
						TestApp: testAppD,
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
				Suites: []Suite{
					{
						Name:    "suite with invalid apps",
						App:     "/path/to/app.zip",
						TestApp: testAppD,
						Devices: []config.Device{
							{Name: "iPhone.*"},
						},
					},
				},
			},
			expectedErr: errors.New("invalid application file: /path/to/app.zip, make sure extension is one of the following: .app, .ipa"),
		},
		{
			name: "validating error with test app other than .ipa / .app",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					{
						App:     appF,
						TestApp: "/path/to/app.zip",
						Devices: []config.Device{
							{Name: "iPhone.*"},
						},
					},
				},
			},
			expectedErr: errors.New("invalid test application file: /path/to/app.zip, make sure extension is one of the following: .app, .ipa"),
		},
		{
			name: "validating throws error on empty testApp",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					{
						Name:    "missing test app",
						App:     appF,
						TestApp: "",
						Devices: []config.Device{
							{Name: "iPhone.*"},
						},
					},
				},
			},
			expectedErr: errors.New("missing path to test app .ipa"),
		},
		{
			name: "validating throws error on not test app .ipa",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					{
						App:     appF,
						TestApp: "/path/to/bundle/tests",
						Devices: []config.Device{
							{Name: "iPhone.*"},
						},
					},
				},
			},
			expectedErr: errors.New("invalid test application file: /path/to/bundle/tests, make sure extension is one of the following: .app, .ipa"),
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
				Suites: []Suite{
					{
						Name:    "no devices",
						App:     appF,
						TestApp: testAppF,
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
				Suites: []Suite{
					{
						Name:    "no device name",
						App:     appF,
						TestApp: testAppF,
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
				Suites: []Suite{
					{
						Name:    "unsupported device type",
						App:     appF,
						TestApp: testAppF,
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
		{
			name: "throws error if devices and simulators are defined",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					{
						Name:    "",
						App:     appF,
						TestApp: testAppF,
						Simulators: []config.Simulator{
							{
								Name:             "iPhone 12 Simulator",
								PlatformName:     "iOS",
								PlatformVersions: []string{"16.2"},
							},
						},
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
			expectedErr: errors.New("suite cannot have both simulators and devices"),
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
		name              string
		project           Project
		testListContent   string
		needsTestListFile bool
		expSuites         []Suite
		expErr            bool
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
			testListContent:   "test1\ntest2\n",
			needsTestListFile: true,
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
			name: "shards suite by testList",
			project: Project{
				Sauce: config.SauceConfig{
					Concurrency: 1,
				},
				Suites: []Suite{
					{
						Name:  "sharding test",
						Shard: "testList",
					},
				},
			},
			testListContent:   "test1\ntest2\n",
			needsTestListFile: true,
			expSuites: []Suite{
				{
					Name: "sharding test - test1",
					TestOptions: TestOptions{
						Class: []string{"test1"},
					},
				},
				{
					Name: "sharding test - test2",
					TestOptions: TestOptions{
						Class: []string{"test2"},
					},
				},
			},
		},
		{
			name: "should ignore empty lines and spaces in testListFile when sharding is enabled",
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
			testListContent:   "   test1\t\n\ntest2\t\n\n",
			needsTestListFile: true,
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
			name: "should return error when sharding w/o a testListFile",
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
			needsTestListFile: false,
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
			name: "should return error when sharding w/ an empty testListFile",
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
			needsTestListFile: true,
			testListContent:   "",
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
			var testListFile string
			if tc.needsTestListFile {
				testListFile = createTestListFile(t, tc.testListContent)
				tc.project.Suites[0].TestListFile = testListFile
			}
			err := ShardSuites(&tc.project)
			if err != nil {
				assert.True(t, tc.expErr)
			}
			for i, s := range tc.project.Suites {
				if diff := cmp.Diff(tc.expSuites[i].TestOptions, s.TestOptions); diff != "" {
					t.Errorf("shard by testList error (-want +got): %s", diff)
				}
				assert.Equal(t, s.Name, tc.expSuites[i].Name)
			}
		})
	}
}

func createTestListFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "tests.txt")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Setup failed: could not write tests.txt: %v", err)
		return ""
	}
	return file
}
