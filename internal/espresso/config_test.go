package espresso

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidateThrowsErrors(t *testing.T) {
	testCases := []struct {
		name        string
		p           *Project
		expectedErr error
	}{
		{
			name:        "validating throws error on empty app",
			p:           &Project{},
			expectedErr: errors.New("missing path to app .apk"),
		},
		{
			name:        "validating throws error on app missing .apk",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app",
				},
			},
			expectedErr: errors.New("invaild application file: /path/to/app, make sure extension is .apk"),
		},
		{
			name:        "validating throws error on empty app",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app.apk",
				},
			},
			expectedErr: errors.New("missing path to test app .apk"),
		},
		{
			name:        "validating throws error on test app missing .apk",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app.apk",
					TestApp: "/path/to/testApp",
				},
			},
			expectedErr: errors.New("invaild test application file: /path/to/testApp, make sure extension is .apk"),
		},
		{
			name:        "validating throws error on missing suites",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app.apk",
					TestApp: "/path/to/testApp.apk",
				},
			},
			expectedErr: errors.New("no suites defined"),
		},
		{
			name:        "validating throws error on missing devices",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app.apk",
					TestApp: "/path/to/testApp.apk",
				},
				Suites: []Suite{
					Suite{
						Name: "no devices",
						Devices: []config.Device{},
					},
				},
			},
			expectedErr: errors.New("missing devices configuration for suite: no devices"),
		},
		{
			name:        "validating throws error on missing device name",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app.apk",
					TestApp: "/path/to/testApp.apk",
				},
				Suites: []Suite{
					Suite{
						Name: "empty device name",
						Devices: []config.Device{
							config.Device{
								Name: "",
							},
						},
					},
				},
			},
			expectedErr: errors.New("missing device name or ID for suite: empty device name. Devices index: 0"),
		},
		{
			name:        "validating throws error on missing Emulator suffix on device name",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app.apk",
					TestApp: "/path/to/testApp.apk",
				},
				Suites: []Suite{
					Suite{
						Name: "no emulator device name",
						Devices: []config.Device{
							config.Device{
								Name: "Android GoogleApi something",
							},
						},
					},
				},
			},
			expectedErr: errors.New("missing `emulator` in device name: Android GoogleApi something, real device cloud is unsupported right now"),
		},
		{
			name:        "validating throws error on missing platform versions",
			p:           &Project{
				Espresso: Espresso{
					App: "/path/to/app.apk",
					TestApp: "/path/to/testApp.apk",
				},
				Suites: []Suite{
					Suite{
						Name: "no emulator device name",
						Devices: []config.Device{
							config.Device{
								Name: "Android GoogleApi Emulator",
								PlatformVersions: []string{},
							},
						},
					},
				},
			},
			expectedErr: errors.New("missing platform versions for device: Android GoogleApi Emulator"),
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
		Espresso: Espresso{
			App: "./tests/apps/calc.apk",
			TestApp: "./tests/apps/calc-success.apk",
		},
		Suites: []Suite{
			{
				Name: "saucy barista",
				Devices: []config.Device{
					{
						Name: "Google Pixel C GoogleAPI Emulator",
						PlatformName: Android,
						PlatformVersions: []string{
							"8.1",
						},
					},
				}},
		},
	}
	if !reflect.DeepEqual(cfg.Espresso, expected.Espresso) {
		t.Errorf("expected: %v, got: %v", expected, cfg)
	}
	if !reflect.DeepEqual(cfg.Suites, expected.Suites) {
		t.Errorf("expected: %v, got: %v", expected, cfg)
	}


}