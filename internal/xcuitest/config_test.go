package xcuitest

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"

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
			name: "validating passing with .ipa",
			p: &Project{
				Xcuitest: Xcuitest{
					App:     "/path/to/app.ipa",
					TestApp: "/path/to/app.ipa",
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
				Xcuitest: Xcuitest{
					App:     "/path/to/app.app",
					TestApp: "/path/to/app.app",
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
				Xcuitest: Xcuitest{
					App:     "/path/to/app.zip",
					TestApp: "/path/to/app.app",
				},
			},
			expectedErr: errors.New("invalid application file: /path/to/app.zip, make sure extension is .ipa or .app"),
		},
		{
			name: "validating error with test app other than .ipa / .app",
			p: &Project{
				Xcuitest: Xcuitest{
					App:     "/path/to/app.ipa",
					TestApp: "/path/to/app.zip",
				},
			},
			expectedErr: errors.New("invalid application test file: /path/to/app.zip, make sure extension is .ipa or .app"),
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
			expectedErr: errors.New("invalid application test file: /path/to/bundle/tests, make sure extension is .ipa or .app"),
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
