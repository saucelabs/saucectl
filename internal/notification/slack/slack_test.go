package slack

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
)

func TestShouldSendNotification(t *testing.T) {
	type params struct {
		jobID  string
		config config.Notifications
		passed bool
	}

	testCases := []struct {
		name     string
		params   params
		expected bool
	}{
		{
			name:     "empty jobID",
			params:   params{jobID: ""},
			expected: false,
		},
		{
			name: "empty slack token",
			params: params{
				jobID:  "123",
				config: config.Notifications{config.Slack{Token: ""}},
			},
			expected: false,
		},
		{
			name: "empty slack channel",
			params: params{
				jobID:  "123",
				config: config.Notifications{config.Slack{Token: "123", Channels: []string{}}},
			},
			expected: false,
		},
		{
			name: "send always",
			params: params{
				jobID:  "123",
				config: config.Notifications{config.Slack{Token: "123", Channels: []string{"test-channel"}, Send: config.SendAlways}},
				passed: true,
			},
			expected: true,
		},
		{
			name: "send on failure",
			params: params{
				jobID:  "123",
				config: config.Notifications{config.Slack{Token: "123", Channels: []string{"test-channel"}, Send: config.SendOnFailure}},
				passed: false,
			},
			expected: true,
		},
		{
			name: "default",
			params: params{
				jobID:  "123",
				config: config.Notifications{config.Slack{Token: "123", Channels: []string{"test-channel"}}},
				passed: true,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		got := ShouldSendNotification(tc.params.jobID, tc.params.passed, tc.params.config)
		if tc.expected != got {
			t.Errorf("test case name: %s  got: %v expected: %v", tc.name, got, tc.expected)
		}
	}
}
