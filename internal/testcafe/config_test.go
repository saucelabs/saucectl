package testcafe

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSetDefaultValues(t *testing.T) {
	s := Suite{
		Speed:            0,
		SelectorTimeout:  0,
		AssertionTimeout: 0,
		PageLoadTimeout:  0,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, float64(1))
	assert.Equal(t, s.SelectorTimeout, 10000)
	assert.Equal(t, s.AssertionTimeout, 3000)
	assert.Equal(t, s.PageLoadTimeout, 3000)

	s = Suite{
		Speed: 2,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, float64(1))

	s = Suite{
		Speed: 0.5,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, 0.5)

	s = Suite{
		Speed:            0,
		SelectorTimeout:  -1,
		AssertionTimeout: -1,
		PageLoadTimeout:  -1,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, float64(1))
	assert.Equal(t, s.SelectorTimeout, 10000)
	assert.Equal(t, s.AssertionTimeout, 3000)
	assert.Equal(t, s.PageLoadTimeout, 3000)
}

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
