package testcafe

import (
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
