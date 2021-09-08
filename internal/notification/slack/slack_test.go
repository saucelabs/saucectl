package slack

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/report"

	"github.com/saucelabs/saucectl/internal/config"
)

func Test_shouldSendNotification(t *testing.T) {
	type params struct {
		testResults []report.TestResult
		config      config.Notifications
		passed      bool
	}

	testCases := []struct {
		name     string
		params   params
		expected bool
	}{
		{
			name: "empty slack channel",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{}}},
			},
			expected: false,
		},
		{
			name: "send always",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}, Send: config.WhenAlways}},
				passed:      true,
			},
			expected: true,
		},
		{
			name: "send pass",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}, Send: config.WhenPass}},
				passed:      true,
			},
			expected: true,
		},
		{
			name: "send on fail",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}, Send: config.WhenFail}},
				passed:      false,
			},
			expected: true,
		},
		{
			name: "default",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}}},
				passed:      true,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		notifier := Reporter{
			Config:      tc.params.config,
			TestResults: tc.params.testResults,
		}
		got := notifier.shouldSendNotification(tc.params.passed)
		if tc.expected != got {
			t.Errorf("test case name: %s  got: %v expected: %v", tc.name, got, tc.expected)
		}
	}
}
