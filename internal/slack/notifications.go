package slack

import "github.com/saucelabs/saucectl/internal/config"

// ShouldSendNotification returns true if it should send notification, otherwise false
func ShouldSendNotification(jobID string, passed bool, cfg config.Notifications) bool {
	if jobID == "" {
		return false
	}

	if cfg.Slack.Token == "" {
		return false
	}

	if cfg.Slack.Channel == "" {
		return false
	}

	if cfg.Slack.Send == config.SendAlways {
		return true
	}

	if cfg.Slack.Send == config.SendOnFailure && !passed {
		return true
	}

	return false
}
