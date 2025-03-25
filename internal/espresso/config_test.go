package espresso

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestValidateThrowsErrors(t *testing.T) {
	dir := fs.NewDir(t, "espresso",
		fs.WithFile("test.apk", "", fs.WithMode(0655)))
	defer dir.Remove()
	appAPK := filepath.Join(dir.Path(), "test.apk")

	testCases := []struct {
		name        string
		p           *Project
		expectedErr error
	}{
		{
			name:        "validating throws error on empty app",
			p:           &Project{Sauce: config.SauceConfig{Region: "us-west-1"}},
			expectedErr: errors.New("missing path to app. Define a path to an .apk or .aab file in the espresso.app property of your config"),
		},
		{
			name: "validating throws error on app missing .apk",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App: "/path/to/app",
				},
			},
			expectedErr: errors.New("invalid application file: /path/to/app, make sure extension is one of the following: .apk, .aab"),
		},
		{
			name: "validating throws error on empty app",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App: appAPK,
				},
			},
			expectedErr: errors.New("missing path to test app. Define a path to an .apk or .aab file in the espresso.testApp property of your config"),
		},
		{
			name: "validating throws error on test app missing .apk",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App:     appAPK,
					TestApp: "/path/to/testApp",
				},
			},
			expectedErr: errors.New("invalid test application file: /path/to/testApp, make sure extension is one of the following: .apk, .aab"),
		},
		{
			name: "validating throws error on missing suites",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App:     appAPK,
					TestApp: appAPK,
				},
			},
			expectedErr: errors.New("no suites defined"),
		},
		{
			name: "validating throws error on missing devices",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App:     appAPK,
					TestApp: appAPK,
				},
				Suites: []Suite{
					{
						Name:    "no devices",
						Devices: []config.Device{},
					},
				},
			},
			expectedErr: errors.New("missing devices or emulators configuration for suite: no devices"),
		},
		{
			name: "validating throws error on missing device name",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App:     appAPK,
					TestApp: appAPK,
				},
				Suites: []Suite{
					{
						Name: "empty emulator name",
						Emulators: []config.Emulator{
							{
								Name: "",
							},
						},
					},
				},
			},
			expectedErr: errors.New("missing emulator name for suite: empty emulator name. Emulators index: 0"),
		},
		{
			name: "validating throws error on missing Emulator suffix on device name",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App:     appAPK,
					TestApp: appAPK,
				},
				Suites: []Suite{
					{
						Name: "no emulator device name",
						Emulators: []config.Emulator{
							{
								Name: "Android GoogleApi something",
							},
						},
					},
				},
			},
			expectedErr: errors.New(`missing "emulator" in emulator name: Android GoogleApi something. Suite name: no emulator device name. Emulators index: 0`),
		},
		{
			name: "validating throws error on missing platform versions",
			p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Espresso: Espresso{
					App:     appAPK,
					TestApp: appAPK,
				},
				Suites: []Suite{
					{
						Name: "no emulator device name",
						Emulators: []config.Emulator{
							{
								Name:             "Android GoogleApi Emulator",
								PlatformVersions: []string{},
							},
						},
					},
				},
			},
			expectedErr: errors.New("missing platform versions for emulator: Android GoogleApi Emulator. Suite name: no emulator device name. Emulators index: 0"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(*tc.p)
			assert.NotNil(t, err)
			assert.Equal(t, tc.expectedErr.Error(), err.Error())
		})
	}
}

func TestFromFile(t *testing.T) {
	dir := fs.NewDir(t, "espresso-cfg",
		fs.WithFile("config.yml", `
apiVersion: v1alpha
kind: espresso
espresso:
  app: ./tests/apps/mda-1.0.17-20.apk
  testApp: ./tests/apps/mda-androidTest-1.0.17-20.apk
suites:
  - name: "saucy barista"
    appSettings:
      audioCapture: true
      resigningEnabled: false
      instrumentation:
        networkCapture: true
        vitals: false
        imageInjection: false
        bypassScreenshotRestriction: true
    devices:
      - name: "Device name"
        platformVersion: 8.1
        options:
          deviceType: TABLET
    emulators:
      - name: "Google Pixel C GoogleAPI Emulator"
        platformVersions:
          - "8.1"
`, fs.WithMode(0655)))
	defer dir.Remove()

	cfg, err := FromFile(filepath.Join(dir.Path(), "config.yml"))
	if err != nil {
		t.Errorf("expected error: %v, got: %v", nil, err)
	}
	trueValue := true
	falseValue := false
	expected := Project{
		ConfigFilePath: filepath.Join(dir.Path(), "config.yml"),
		Espresso: Espresso{
			App:     "./tests/apps/mda-1.0.17-20.apk",
			TestApp: "./tests/apps/mda-androidTest-1.0.17-20.apk",
		},
		Suites: []Suite{
			{
				Name: "saucy barista",
				AppSettings: config.AppSettings{
					ResigningEnabled: &falseValue,
					AudioCapture:     &trueValue,
					Instrumentation: config.Instrumentation{
						ImageInjection:              &falseValue,
						BypassScreenshotRestriction: &trueValue,
						Biometrics:                  nil,
						Vitals:                      &falseValue,
						NetworkCapture:              &trueValue,
					},
				},
				Devices: []config.Device{
					{
						Name:            "Device name",
						PlatformVersion: "8.1",
						Options: config.DeviceOptions{
							DeviceType: "TABLET",
						},
					},
				},
				Emulators: []config.Emulator{
					{
						Name: "Google Pixel C GoogleAPI Emulator",
						PlatformVersions: []string{
							"8.1",
						},
					},
				}},
		},
	}
	diff := cmp.Diff(cfg.Espresso, expected.Espresso)
	if diff != "" {
		t.Errorf("differences found: %v", diff)
	}
	diff = cmp.Diff(cfg.Suites, expected.Suites)
	if diff != "" {
		t.Errorf("differences found: %v", diff)
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
				Espresso: Espresso{
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
				Espresso: Espresso{
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

func TestEspresso_SortByHistory(t *testing.T) {
	testCases := []struct {
		name    string
		suites  []Suite
		history insights.JobHistory
		expRes  []Suite
	}{
		{
			name: "sort suites by job history",
			suites: []Suite{
				{Name: "suite 1"},
				{Name: "suite 2"},
				{Name: "suite 3"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					{Name: "suite 2"},
					{Name: "suite 1"},
					{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				{Name: "suite 2"},
				{Name: "suite 1"},
				{Name: "suite 3"},
			},
		},
		{
			name: "suites is the subset of job history",
			suites: []Suite{
				{Name: "suite 1"},
				{Name: "suite 2"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					{Name: "suite 2"},
					{Name: "suite 1"},
					{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				{Name: "suite 2"},
				{Name: "suite 1"},
			},
		},
		{
			name: "job history is the subset of suites",
			suites: []Suite{
				{Name: "suite 1"},
				{Name: "suite 2"},
				{Name: "suite 3"},
				{Name: "suite 4"},
				{Name: "suite 5"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					{Name: "suite 2"},
					{Name: "suite 1"},
					{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				{Name: "suite 2"},
				{Name: "suite 1"},
				{Name: "suite 3"},
				{Name: "suite 4"},
				{Name: "suite 5"},
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
