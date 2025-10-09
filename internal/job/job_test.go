package job

import (
	"testing"
)

func TestJob_TotalStatus(t *testing.T) {
	testCases := []struct {
		name           string
		job            Job
		expectedStatus string
	}{
		{
			name: "RDC job with Completed flag should return complete",
			job: Job{
				Status:    StateComplete,
				Completed: true,
				Passed:    false,
				IsRDC:     true,
			},
			expectedStatus: StateComplete,
		},
		{
			name: "RDC job with Completed and Passed flags should return complete (Completed takes precedence for RDC)",
			job: Job{
				Status:    StateComplete,
				Completed: true,
				Passed:    true,
				IsRDC:     true,
			},
			expectedStatus: StateComplete,
		},
		{
			name: "RDC job with only Passed flag should return passed",
			job: Job{
				Status:    StatePassed,
				Completed: false,
				Passed:    true,
				IsRDC:     true,
			},
			expectedStatus: StatePassed,
		},
		{
			name: "VDC job with Passed flag should return passed",
			job: Job{
				Status:    StateComplete,
				Completed: false,
				Passed:    true,
				IsRDC:     false,
			},
			expectedStatus: StatePassed,
		},
		{
			name: "VDC job with both Completed and Passed should return passed (Passed takes precedence for VDC)",
			job: Job{
				Status:    StateComplete,
				Completed: true,
				Passed:    true,
				IsRDC:     false,
			},
			expectedStatus: StatePassed,
		},
		{
			name: "VDC job with only Completed flag should return failed",
			job: Job{
				Status:    StateComplete,
				Completed: true,
				Passed:    false,
				IsRDC:     false,
			},
			expectedStatus: StateFailed,
		},
		{
			name: "Job with failed status should return failed",
			job: Job{
				Status:    StateFailed,
				Completed: false,
				Passed:    false,
				IsRDC:     false,
			},
			expectedStatus: StateFailed,
		},
		{
			name: "Job with error status should return failed",
			job: Job{
				Status:    StateError,
				Completed: false,
				Passed:    false,
				IsRDC:     false,
			},
			expectedStatus: StateFailed,
		},
		{
			name: "Job in progress should return in progress",
			job: Job{
				Status:    StateInProgress,
				Completed: false,
				Passed:    false,
				IsRDC:     false,
			},
			expectedStatus: StateInProgress,
		},
		{
			name: "Job queued should return queued",
			job: Job{
				Status:    StateQueued,
				Completed: false,
				Passed:    false,
				IsRDC:     false,
			},
			expectedStatus: StateQueued,
		},
		{
			name: "RDC job not done should return current status",
			job: Job{
				Status:    StateInProgress,
				Completed: false,
				Passed:    false,
				IsRDC:     true,
			},
			expectedStatus: StateInProgress,
		},
		{
			name: "Playwright test passed (VDC) should return passed",
			job: Job{
				Status:    StateComplete,
				Completed: true,
				Passed:    true,
				IsRDC:     false,
				Framework: "playwright",
			},
			expectedStatus: StatePassed,
		},
		{
			name: "Playwright test completed but not passed (VDC) should return failed",
			job: Job{
				Status:    StateComplete,
				Completed: true,
				Passed:    false,
				IsRDC:     false,
				Framework: "playwright",
			},
			expectedStatus: StateFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.job.TotalStatus()
			if got != tc.expectedStatus {
				t.Errorf("TotalStatus() = %v, want %v", got, tc.expectedStatus)
			}
		})
	}
}

func TestJob_IsSuccessful(t *testing.T) {
	testCases := []struct {
		name     string
		job      Job
		expected bool
	}{
		{
			name: "VDC job with Passed flag is successful",
			job: Job{
				Passed:    true,
				Completed: false,
				IsRDC:     false,
			},
			expected: true,
		},
		{
			name: "VDC job with only Completed flag is NOT successful",
			job: Job{
				Passed:    false,
				Completed: true,
				IsRDC:     false,
			},
			expected: false,
		},
		{
			name: "VDC job with both Passed and Completed is successful",
			job: Job{
				Passed:    true,
				Completed: true,
				IsRDC:     false,
			},
			expected: true,
		},
		{
			name: "VDC job with neither Passed nor Completed is not successful",
			job: Job{
				Passed:    false,
				Completed: false,
				IsRDC:     false,
			},
			expected: false,
		},
		{
			name: "RDC job with only Completed is successful",
			job: Job{
				Passed:    false,
				Completed: true,
				IsRDC:     true,
			},
			expected: true,
		},
		{
			name: "RDC job with only Passed is successful",
			job: Job{
				Passed:    true,
				Completed: false,
				IsRDC:     true,
			},
			expected: true,
		},
		{
			name: "RDC job with both Passed and Completed is successful",
			job: Job{
				Passed:    true,
				Completed: true,
				IsRDC:     true,
			},
			expected: true,
		},
		{
			name: "RDC job with neither Passed nor Completed is not successful",
			job: Job{
				Passed:    false,
				Completed: false,
				IsRDC:     true,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.job.IsSuccessful()
			if got != tc.expected {
				t.Errorf("IsSuccessful() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestDone(t *testing.T) {
	testCases := []struct {
		name     string
		status   string
		expected bool
	}{
		{
			name:     "StateComplete is done",
			status:   StateComplete,
			expected: true,
		},
		{
			name:     "StateError is done",
			status:   StateError,
			expected: true,
		},
		{
			name:     "StatePassed is done",
			status:   StatePassed,
			expected: true,
		},
		{
			name:     "StateFailed is done",
			status:   StateFailed,
			expected: true,
		},
		{
			name:     "StateInProgress is not done",
			status:   StateInProgress,
			expected: false,
		},
		{
			name:     "StateQueued is not done",
			status:   StateQueued,
			expected: false,
		},
		{
			name:     "StateNew is not done",
			status:   StateNew,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Done(tc.status)
			if got != tc.expected {
				t.Errorf("Done(%v) = %v, want %v", tc.status, got, tc.expected)
			}
		})
	}
}
