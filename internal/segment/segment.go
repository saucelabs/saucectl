package segment

import (
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/setup"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/version"
	"gopkg.in/segmentio/analytics-go.v3"
	"runtime"
)

type Tracker struct {
	client analytics.Client
}

func New() *Tracker {
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
			Extra: map[string]interface{}{"ci": ci.GetProvider().Name},
		},
	})
	if err != nil {
		// Usage is not crucial to the execution of saucectl, so proceed without notifying or blocking the user.
		log.Debug().Err(err).Msg("Failed to create segment client")
		return &Tracker{}
	}

	return &Tracker{client: client}
}

func (t *Tracker) Collect(subject string, props usage.Properties) {
	if t.client == nil {
		return
	}

	p := analytics.NewProperties()
	p.Set("subject_name", subject).Set("product_area", "DevX").
		Set("product_sub_area", "SauceCTL")

	for k, v := range props {
		p[k] = v
	}

	userId := credentials.Get().Username
	if userId == "" {
		userId = "saucectlanon"
	}

	if err := t.client.Enqueue(analytics.Track{
		UserId:     userId,
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

func (t *Tracker) Close() error {
	return t.client.Close()
}
