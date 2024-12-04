package usage

import (
	"runtime"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/setup"
	"github.com/saucelabs/saucectl/internal/version"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/segmentio/analytics-go.v3"
)

// DefaultClient is the default preconfigured instance of Client.
var DefaultClient = NewClient(true)

// Client is a thin wrapper around analytics.Client.
type Client struct {
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

// NewClient creates a new instance of Client.
func NewClient(enabled bool) *Client {
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
		return &Client{}
	}

	return &Client{client: client, Enabled: enabled}
}

// Collect reports the usage of subject along with its attached metadata that is props.
func (c *Client) Collect(subject string, opts ...Option) {
	if !c.Enabled {
		return
	}
	if c.client == nil {
		return
	}

	p := analytics.NewProperties()
	p.Set("subject_name", cases.Title(language.English).String(subject)).
		Set("product_area", "DevX").
		Set("product_sub_area", "SauceCTL").
		Set("ci", ci.GetProvider().Name)

	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}

	userID := credentials.Get().Username
	if userID == "" {
		userID = "saucectlanon"
	}

	if err := c.client.Enqueue(analytics.Track{
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
func (c *Client) Close() error {
	return c.client.Close()
}
