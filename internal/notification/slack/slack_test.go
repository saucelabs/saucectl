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
		token       string
		passed      bool
	}

	testCases := []struct {
		name     string
		params   params
		expected bool
	}{
		{
			name:     "empty URL",
			params:   params{testResults: []report.TestResult{report.TestResult{URL: ""}}, token: "token"},
			expected: false,
		},
		{
			name:     "empty token",
			params:   params{testResults: []report.TestResult{report.TestResult{URL: ""}}, token: ""},
			expected: false,
		},
		{
			name: "empty slack channel",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{}}},
				token:       "token",
			},
			expected: false,
		},
		{
			name: "send always",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}, Send: config.WhenAlways}},
				passed:      true,
				token:       "token",
			},
			expected: true,
		},
		{
			name: "send pass",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}, Send: config.WhenPass}},
				passed:      true,
				token:       "token",
			},
			expected: true,
		},
		{
			name: "send on fail",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}, Send: config.WhenFail}},
				passed:      false,
				token:       "token",
			},
			expected: true,
		},
		{
			name: "default",
			params: params{
				testResults: []report.TestResult{report.TestResult{URL: "http://example.com"}},
				config:      config.Notifications{config.Slack{Channels: []string{"test-channel"}}},
				passed:      true,
				token:       "token",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		notifier := Reporter{
			Config:      tc.params.config,
			TestResults: tc.params.testResults,
			Token:       tc.params.token,
		}
		got := notifier.shouldSendNotification(tc.params.passed)
		if tc.expected != got {
			t.Errorf("test case name: %s  got: %v expected: %v", tc.name, got, tc.expected)
		}
	}
}
