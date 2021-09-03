package notification

import "github.com/saucelabs/saucectl/internal/report"

// Reporter represents common interface for sending notifications.
type Reporter interface {
	report.Reporter
	SendMessage(passed bool)
}
