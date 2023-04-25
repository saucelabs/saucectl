package saucereport

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFailedTests(t *testing.T) {
	testcases := []struct {
		name     string
		input    SauceReport
		expected []string
	}{
		{
			name: "Sauce Report doesn't have failed tests",
			input: SauceReport{
				Status: StatusPassed,
				Suites: []Suite{
					{
						Name:   "first suite",
						Status: StatusPassed,
						Tests: []Test{
							{
								Name:   "first test",
								Status: StatusPassed,
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Sauce Report has failed tests, suites only have tests",
			input: SauceReport{
				Status: StatusFailed,
				Suites: []Suite{
					{
						Name:   "first suite",
						Status: StatusFailed,
						Tests: []Test{
							{
								Name:   "first test",
								Status: StatusFailed,
							},
						},
					},
				},
			},
			expected: []string{"first test"},
		},
		{
			name: "Sauce Report has failed tests, suites have suites and tests",
			input: SauceReport{
				Status: StatusFailed,
				Suites: []Suite{
					{
						Name:   "first suite",
						Status: StatusFailed,
						Tests: []Test{
							{
								Name:   "first test",
								Status: StatusFailed,
							},
						},
						Suites: []Suite{
							{
								Name:   "second suite",
								Status: StatusFailed,
								Tests: []Test{
									{
										Name:   "second test",
										Status: StatusFailed,
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"first test", "second test"},
		},
		{
			name: "Sauce Report has failed tests, suites have empty tests",
			input: SauceReport{
				Status: StatusFailed,
				Suites: []Suite{
					{
						Name:   "first suite",
						Status: StatusFailed,
						Tests:  []Test{},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Sauce Report has failed tests, suites have empty suites",
			input: SauceReport{
				Status: StatusFailed,
				Suites: []Suite{
					{
						Name:   "first suite",
						Status: StatusFailed,
						Suites: []Suite{},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Sauce Report has passed and failed suites",
			input: SauceReport{
				Status: StatusFailed,
				Suites: []Suite{
					{
						Name:   "first suite",
						Status: StatusFailed,
						Tests: []Test{
							{
								Name:   "failed test",
								Status: StatusFailed,
							},
						},
					},
					{
						Name:   "second suite",
						Status: StatusPassed,
						Tests: []Test{
							{
								Name:   "passed test",
								Status: StatusPassed,
							},
						},
					},
				},
			},
			expected: []string{"failed test"},
		},
		{
			name: "Sauce Report has skipped suites",
			input: SauceReport{
				Status: StatusFailed,
				Suites: []Suite{
					{
						Name:   "first suite",
						Status: StatusSkipped,
						Tests: []Test{
							{
								Name:   "skipped test",
								Status: StatusSkipped,
							},
						},
					},
					{
						Name:   "second suite",
						Status: StatusFailed,
						Tests: []Test{
							{
								Name:   "failed test",
								Status: StatusFailed,
							},
						},
					},
				},
			},
			expected: []string{"failed test"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetFailedTests(tc.input)
			sort.Strings(result)
			assert.Equal(t, tc.expected, result)
		})
	}
}
