package segment

import (
	"github.com/saucelabs/saucectl/internal/credentials"
	"runtime"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/setup"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/version"
	"gopkg.in/segmentio/analytics-go.v3"
)

// DefaultTracker is the default Tracker.
var DefaultTracker = New(true)

// Tracker is the segment implementation for usage.Tracker.
type Tracker struct {
	client  analytics.Client
	Enabled bool
}

// debugLogger is a logger that redirects logs to the debug log.
type debugLogger struct{}

func (l debugLogger) Logf(format string, args ...interface{}) {
	log.Debug().Msgf(format, args...)
}

func (l debugLogger) Errorf(format string, args ...interface{}) {
	log.Debug().Msgf(format, args...)
}

// New creates a new instance of Tracker.
func New(enabled bool) *Tracker {
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
		Logger: debugLogger{},
	})
	if err != nil {
		// Usage is not crucial to the execution of saucectl, so proceed without notifying or blocking the user.
		log.Debug().Err(err).Msg("Failed to create segment client")
		return &Tracker{}
	}

	return &Tracker{client: client, Enabled: enabled}
}

// Collect reports the usage of subject along with its attached metadata that is props.
func (t *Tracker) Collect(subject string, opts ...usage.Option) {
	if !t.Enabled {
		return
	}
	if t.client == nil {
		return
	}

	p := analytics.NewProperties()
	p.Set("subject_name", cases.Title(language.English).String(subject)).
		Set("product_area", "DevX").
		Set("product_sub_area", "SauceCTL").
		Set("ci", ci.GetProvider().Name)

	for _, opt := range opts {
		opt(p)
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
			"Pendo":     true,
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
