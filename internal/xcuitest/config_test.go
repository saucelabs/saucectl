package xcuitest

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"

	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestValidate(t *testing.T) {
	dir := fs.NewDir(t, "xcuitest-config",
		fs.WithFile("test.ipa", "", fs.WithMode(0655)),
		fs.WithFile("testApp.ipa", "", fs.WithMode(0655)))
	defer dir.Remove()
	appF := filepath.Join(dir.Path(), "test.ipa")
	testAppF := filepath.Join(dir.Path(), "testApp.ipa")

	testCases := []struct {
		name        string
		p           *Project
		expectedErr error
	}{
		{
			name:        "validating throws error on empty app",
			p:           &Project{},
			expectedErr: errors.New("missing path to app .ipa"),
		},
		{
			name: "validating throws error on app missing .ipa",
			p: &Project{
				Xcuitest: Xcuitest{
					App: "/path/to/app",
				},
			},
			expectedErr: errors.New("invalid application file: /path/to/app, make sure extension is .ipa"),
		},
		{
			name: "validating throws error on empty testApp",
			p: &Project{
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
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: "/path/to/bundle/tests",
				},
			},
			expectedErr: errors.New("invalid application test file: /path/to/bundle/tests, make sure extension is .ipa"),
		},
		{
			name: "validating throws error on missing suites",
			p: &Project{
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
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
				Suites: []Suite{
					Suite{
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
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
				Suites: []Suite{
					Suite{
						Name: "no device name",
						Devices: []config.Device{
							{
								Name: "",
							},
						},
					},
				},
			},
			expectedErr: errors.New("missing device name or id for suite: no device name. Devices index: 0"),
		},
		{
			name: "validating throws error on unsupported device type",
			p: &Project{
				Xcuitest: Xcuitest{
					App:     appF,
					TestApp: testAppF,
				},
				Suites: []Suite{
					Suite{
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
			assert.NotNil(t, err)
			assert.Equal(t, err.Error(), tc.expectedErr.Error())
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

func TestSetDeviceDefaultValues(t *testing.T) {
	p := Project{
		Suites: []Suite{
			{
				Name: "test suite 1",
				Devices: []config.Device{
					{
						Name: "iPhone 11",
						Options: config.DeviceOptions{
							DeviceType: "phone",
						},
					},
					{
						Name: "iPhone XR",
						Options: config.DeviceOptions{
							DeviceType: "tablet",
						},
					},
				},
			},
		},
	}

	SetDeviceDefaultValues(&p)

	expected := Project{
		Suites: []Suite{
			{
				Name: "test suite 1",
				Devices: []config.Device{
					{
						Name:         "iPhone 11",
						PlatformName: "iOS",
						Options: config.DeviceOptions{
							DeviceType: "PHONE",
						},
					},
					{
						Name:         "iPhone XR",
						PlatformName: "iOS",
						Options: config.DeviceOptions{
							DeviceType: "TABLET",
						},
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(p, expected) {
		t.Errorf("expected: %v, got: %v", expected, p)
	}
}
