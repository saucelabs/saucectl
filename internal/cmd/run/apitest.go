package run

import (
	"os"
	"time"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/usage"

	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/time/rate"
)

// Query rate is queryRequestRate per second.
var queryRequestRate = 1
var rateLimitTokenBucket = 10

func runApitest(cmd *cobra.Command, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := apitest.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	if err := applyApitestFlags(&p); err != nil {
		return 1, err
	}
	apitest.SetDefaults(&p)
	if err := apitest.Validate(p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	restoClient.URL = regio.APIBaseURL()
	apitestingClient.URL = regio.APIBaseURL()

	r := apitest.Runner{
		Project: p,
		Client:  &apitestingClient,
		Region:  regio,
		Reporters: []report.Reporter{
			&table.Reporter{
				Dst: os.Stdout,
			},
		},
		Async:              gFlags.async,
		TunnelService:      &restoClient,
		RequestRateLimiter: rate.NewLimiter(rate.Every(time.Duration(1/queryRequestRate)*time.Second), rateLimitTokenBucket),
	}

	if err := r.ResolveHookIDs(); err != nil {
		return 1, err
	}

	tracker := segment.DefaultTracker

	go func() {
		props := usage.Properties{}
		props.SetFramework("apit")
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	return r.RunProject()
}

func applyApitestFlags(p *apitest.Project) error {
	if gFlags.selectedSuite != "" {
		if err := apitest.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}
	return nil
}
