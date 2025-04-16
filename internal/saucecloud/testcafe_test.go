package saucecloud

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/stretchr/testify/assert"
)

func TestTestcafe_GetSuiteNames(t *testing.T) {
	runner := &TestcafeRunner{
		Project: &testcafe.Project{
			Suites: []testcafe.Suite{
				{Name: "suite1"},
				{Name: "suite2"},
				{Name: "suite3"},
			},
		},
	}

	assert.Equal(t, []string{"suite1", "suite2", "suite3"}, runner.getSuiteNames())
}

func Test_calcTestcafeJobsCount(t *testing.T) {
	testCases := []struct {
		name              string
		suites            []testcafe.Suite
		expectedJobsCount int
	}{
		{
			name: "single suite",
			suites: []testcafe.Suite{
				{
					Name: "single suite",
				},
			},
			expectedJobsCount: 1,
		},
		{
			name: "two suites",
			suites: []testcafe.Suite{
				{
					Name: "first suite",
				},
				{
					Name: "second suite",
				},
			},
			expectedJobsCount: 2,
		},
		{
			name: "suites with simulators and platfrom versions",
			suites: []testcafe.Suite{
				{
					Name: "first suite",
				},
				{
					Name: "second suite",
				},
				{
					Name: "suite with one simulator and two platforms",
					Simulators: []config.Simulator{
						{PlatformVersions: []string{"12.0", "14.3"}},
					},
				},
				{
					Name: "suite with two simulators and two platforms",
					Simulators: []config.Simulator{
						{PlatformVersions: []string{"12.0", "14.3"}},
						{PlatformVersions: []string{"12.0", "14.3"}},
					},
				},
			},
			expectedJobsCount: 8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tr := TestcafeRunner{}
			got := tr.calcTestcafeJobsCount(tc.suites)
			if tc.expectedJobsCount != got {
				t.Errorf("expected: %d, got: %d", tc.expectedJobsCount, got)
			}
		})
	}
}
