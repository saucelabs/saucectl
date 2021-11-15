package segment

import (
	"runtime"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/setup"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/version"
	"gopkg.in/segmentio/analytics-go.v3"
)

// Tracker is the segment implemention for usage.Tracker.
type Tracker struct {
	client     analytics.Client
	IsDisabled bool
}

// New creates a new instance of Tracker.
func New(IsDisabled bool) *Tracker {
	client, err := analytics.NewWithConfig(setup.SegmentWriteKey, analytics.Config{
		BatchSize: 1,
		DefaultContext: &analytics.Context{
			App: analytics.AppInfo{
				Name:    "saucectl",
				Version: version.Version,
			},
			OS: analytics.OSInfo{
				Name: runtime.GOOS + " " + runtime.GOARCH,
			},
		},
	})
	if err != nil {
		// Usage is not crucial to the execution of saucectl, so proceed without notifying or blocking the user.
		log.Debug().Err(err).Msg("Failed to create segment client")
		return &Tracker{}
	}

	return &Tracker{client: client, IsDisabled: IsDisabled}
}

// Collect reports the usage of subject along with its attached metadata that is props.
func (t *Tracker) Collect(subject string, props usage.Properties) {
	if t.IsDisabled {
		return
	}
	if t.client == nil {
		return
	}

	p := analytics.NewProperties()
	p.Set("subject_name", subject).Set("product_area", "DevX").
		Set("product_sub_area", "SauceCTL").Set("ci", ci.GetProvider().Name)

	for k, v := range props {
		p[k] = v
	}

	userID := credentials.Get().Username
	if userID == "" {
		userID = "saucectlanon"
	}

	if err := t.client.Enqueue(analytics.Track{
		UserId:     userID,
		Event:      "Command Executed",
		Properties: p,

		Integrations: map[string]interface{}{
			"All":       false,
			"Mixpanel":  true,
			"Snowflake": true,
		},
	}); err != nil {
		// Usage is not crucial to the execution of saucectl, so proceed without notifying or blocking the user.
		log.Debug().Err(err).Msg("Failed to collect usage metrics")
	}
}

// Close closes the underlying client.
func (t *Tracker) Close() error {
	return t.client.Close()
}
