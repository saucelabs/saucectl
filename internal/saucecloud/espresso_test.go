package saucecloud

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestEspresso_GetSuiteNames(t *testing.T) {
	runner := &EspressoRunner{
		Project: espresso.Project{
			Suites: []espresso.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, "suite1, suite2, suite3", runner.getSuiteNames())
}

func TestEspressoRunner_CalculateJobCount(t *testing.T) {
	runner := &EspressoRunner{
		Project: espresso.Project{
			Espresso: espresso.Espresso{
				App: "/path/to/app.apk",
				TestApp: "/path/to/testApp.apk",
			},
			Suites: []espresso.Suite{
				espresso.Suite{
					Name: "valid espresso project",
					Devices: []config.Device{
						config.Device{
							Name: "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						config.Device{
							Name: "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
			},
		},
	}
	assert.Equal(t, runner.calculateJobsCount(runner.Project.Suites), 3)
}
