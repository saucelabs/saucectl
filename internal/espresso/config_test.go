package espresso

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
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
			expectedErr: errors.New("missing `emulator` in emulator name: Android GoogleApi something. Suite name: no emulator device name. Emulators index: 0"),
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
		fs.WithFile("config.yml", `apiVersion: v1alpha
kind: espresso
espresso:
  app: ./tests/apps/calc.apk
  testApp: ./tests/apps/calc-success.apk
suites:
  - name: "saucy barista"
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
	expected := Project{
		ConfigFilePath: filepath.Join(dir.Path(), "config.yml"),
		Espresso: Espresso{
			App:     "./tests/apps/calc.apk",
			TestApp: "./tests/apps/calc-success.apk",
		},
		Suites: []Suite{
			{
				Name: "saucy barista",
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
	if !reflect.DeepEqual(cfg.Espresso, expected.Espresso) {
		t.Errorf("expected: %v, got: %v", expected.Espresso, cfg.Espresso)
	}
	if !reflect.DeepEqual(cfg.Suites, expected.Suites) {
		t.Errorf("expected: %v, got: %v", expected.Suites, cfg.Suites)
	}

}
