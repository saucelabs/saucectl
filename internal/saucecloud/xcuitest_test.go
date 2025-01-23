package saucecloud

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

func TestCalculateJobsCount(t *testing.T) {
	runner := &XcuitestRunner{
		Project: xcuitest.Project{
			Xcuitest: xcuitest.Xcuitest{
				App:     "/path/to/app.ipa",
				TestApp: "/path/to/testApp.ipa",
			},
			Suites: []xcuitest.Suite{
				{
					Name: "valid xcuitest project",
					Devices: []config.Device{
						{
							Name:            "iPhone 11",
							PlatformName:    "iOS",
							PlatformVersion: "14.3",
						},
						{
							Name:            "iPhone XR",
							PlatformName:    "iOS",
							PlatformVersion: "14.3",
						},
					},
				},
			},
		},
	}
	assert.Equal(t, runner.calculateJobsCount(runner.Project.Suites), 2)
}
