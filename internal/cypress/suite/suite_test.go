package suite

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/insights"
	"gotest.tools/assert"
)

func TestSuite_SortByHistory(t *testing.T) {
	testCases := []struct {
		name    string
		suites  []Suite
		history insights.JobHistory
		expRes  []Suite
	}{
		{
			name: "sort suites by job history",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
				Suite{Name: "suite 3"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
				Suite{Name: "suite 3"},
			},
		},
		{
			name: "suites is the subset of job history",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
			},
		},
		{
			name: "job history is the subset of suites",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
				Suite{Name: "suite 3"},
				Suite{Name: "suite 4"},
				Suite{Name: "suite 5"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
				Suite{Name: "suite 3"},
				Suite{Name: "suite 4"},
				Suite{Name: "suite 5"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SortByHistory(tc.suites, tc.history)
			for i := 0; i < len(result); i++ {
				assert.Equal(t, tc.expRes[i].Name, result[i].Name)
			}
		})
	}
}
