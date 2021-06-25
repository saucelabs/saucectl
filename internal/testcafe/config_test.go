package testcafe

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func Test_appleDeviceRegex(t *testing.T) {
	tests := []struct {
		deviceName string
		want       bool
	}{
		{
			deviceName: "iPhone Simulator",
			want:       true,
		},
		{
			deviceName: "iphone simulator",
			want:       true,
		},
		{
			deviceName: "iPhone SE (2nd generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPhone 8 Simulator",
			want:       true,
		},
		{
			deviceName: "iPhone 8 Plus Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Pro (12.9 inch) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Pro (12.9 inch) (4th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Air Simulator",
			want:       true,
		},
		{
			deviceName: "iPad (8th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad mini (5th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "Android GoogleAPI Emulator",
			want:       false,
		},
		{
			deviceName: "Google Pixel 3a GoogleAPI Emulator",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.deviceName, func(t *testing.T) {
			got := appleDeviceRegex.MatchString(tt.deviceName)
			if got != tt.want {
				t.Errorf("appleDeviceRegex.MatchString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			err := FilterSuites(tc.config, tc.suiteName)
			if err != nil {
				assert.Equal(t, tc.expErr, err.Error())
			}
			assert.True(t, reflect.DeepEqual(*tc.config, tc.expConfig))
		})
	}
}
