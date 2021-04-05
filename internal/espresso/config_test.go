package espresso

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/stretchr/testify/assert"
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
			expectedErr: errors.New("missing device name for suite: empty device name. Devices index: 0"),
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
			assert.Equal(t, err.Error(), tc.expectedErr.Error())
		})
	}
}

func TestValidatePlatformNameIsSet(t *testing.T) {
	p := Project{
		Espresso: Espresso{
			App: "/path/to/app.apk",
			TestApp: "/path/to/testApp.apk",
		},
		Suites: []Suite{
			Suite{
				Name: "valid espresso project",
				Devices: []config.Device{
					config.Device{
						Name: "Android GoogleApi Emulator",
						PlatformVersions: []string{"11.0"},
					},
				},
			},
		},
	}
	err := Validate(p)
	assert.Nil(t, err)
	for _, suite := range p.Suites {
		for _, device := range suite.Devices {
			assert.Equal(t, device.PlatformName, Android)
		}
	}
}
