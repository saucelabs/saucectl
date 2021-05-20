package xcuit

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestValidate(t *testing.T) {
	dir := fs.NewDir(t, "xcuit-config",
		fs.WithFile("test.ipa", "", fs.WithMode(0655)),
		fs.WithDir("test.app", fs.WithMode(0655)))
	defer dir.Remove()
	appF := filepath.Join(dir.Path(), "test.ipa")
	appBundle := filepath.Join(dir.Path(), "test.app")

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
				Xcuit: Xcuit{
					App: "/path/to/app",
				},
			},
			expectedErr: errors.New("invaild application file: /path/to/app, make sure extension is .ipa"),
		},
		{
			name: "validating throws error on not exists .ipa file",
			p: &Project{
				Xcuit: Xcuit{
					App: "/path/to/app/test.ipa",
				},
			},
			expectedErr: errors.New("application file: /path/to/app/test.ipa does not exists"),
		},
		{
			name: "validating throws error on empty testApp",
			p: &Project{
				Xcuit: Xcuit{
					App:     appF,
					TestApp: "",
				},
			},
			expectedErr: errors.New("missing path to the bundle with tests"),
		},
		{
			name: "validating throws error on not exists bundle with tests",
			p: &Project{
				Xcuit: Xcuit{
					App:     appF,
					TestApp: "/path/to/bundle/tests",
				},
			},
			expectedErr: errors.New("bundle with tests: /path/to/bundle/tests does not exists"),
		},
		{
			name: "validating throws error on missing suites",
			p: &Project{
				Xcuit: Xcuit{
					App:     appF,
					TestApp: appBundle,
				},
			},
			expectedErr: errors.New("no suites defined"),
		},
		{
			name: "validating throws error on missing devices",
			p: &Project{
				Xcuit: Xcuit{
					App:     appF,
					TestApp: appBundle,
				},
				Suites: []Suite{
					Suite{
						Name:    "no devices",
						Devices: []Device{},
					},
				},
			},
			expectedErr: errors.New("missing devices configuration for suite: no devices"),
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
	dir := fs.NewDir(t, "xcuit-cfg",
		fs.WithFile("config.yml", `apiVersion: v1alpha
kind: xcuit
xcuit:
  app: "./tests/apps/xcuit/SauceLabs.Mobile.Sample.XCUITest.App.ipa"
  testApp: "./tests/apps/xcuit/SwagLabsMobileAppUITests-Runner.app"
suites:
  - name: "saucy barista"
    devices:
      - name: "iPhone XR"
        platformVersion: "14.3"
    testOptions:
      - class: "SwagLabsMobileAppUITests.LoginTests"
        method: "testSuccessfulLogin"
      - class: "SwagLabsMobileAppUITests.LoginTests"
`, fs.WithMode(0655)))
	defer dir.Remove()

	cfg, err := FromFile(filepath.Join(dir.Path(), "config.yml"))
	if err != nil {
		t.Errorf("expected error: %v, got: %v", nil, err)
	}
	expected := Project{
		Xcuit: Xcuit{
			App:     "./tests/apps/xcuit/SauceLabs.Mobile.Sample.XCUITest.App.ipa",
			TestApp: "./tests/apps/xcuit/SwagLabsMobileAppUITests-Runner.app",
		},
		Suites: []Suite{
			{
				Name: "saucy barista",
				Devices: []Device{
					{
						Name:            "iPhone XR",
						PlatformVersion: "14.3",
					},
				},
				TestOptions: []TestOption{
					TestOption{
						Class:  "SwagLabsMobileAppUITests.LoginTests",
						Method: "testSuccessfulLogin",
					},
					TestOption{
						Class: "SwagLabsMobileAppUITests.LoginTests",
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(cfg.Xcuit, expected.Xcuit) {
		t.Errorf("expected: %v, got: %v", expected, cfg)
	}
	if !reflect.DeepEqual(cfg.Suites, expected.Suites) {
		t.Errorf("expected: %v, got: %v", expected, cfg)
	}
}
