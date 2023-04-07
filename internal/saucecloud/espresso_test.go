package saucecloud

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/espresso"
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
	tests := []struct {
		name   string
		suites []espresso.Suite
		wants  int
	}{
		{
			name: "should multiply emulator combinations",
			suites: []espresso.Suite{
				{
					Name: "valid espresso project",
					Emulators: []config.Emulator{
						{
							Name:             "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						{
							Name:             "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
			},
			wants: 3,
		},
		{
			name:  "should multiply jobs by NumShards if defined",
			wants: 18,
			suites: []espresso.Suite{
				{
					Name: "first suite",
					TestOptions: map[string]interface{}{
						"numShards": 3,
					},
					Emulators: []config.Emulator{
						{
							Name:             "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						{
							Name:             "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
				{
					Name: "second suite",
					TestOptions: map[string]interface{}{
						"numShards": 3,
					},
					Emulators: []config.Emulator{
						{
							Name:             "Android GoogleApi Emulator",
							PlatformVersions: []string{"11.0", "10.0"},
						},
						{
							Name:             "Android Emulator",
							PlatformVersions: []string{"11.0"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		runner := &EspressoRunner{
			Project: espresso.Project{
				Espresso: espresso.Espresso{
					App:     "/path/to/app.apk",
					TestApp: "/path/to/testApp.apk",
				},
				Suites: tt.suites,
			},
		}

		assert.Equal(t, runner.calculateJobsCount(runner.Project.Suites), tt.wants)
	}
}
