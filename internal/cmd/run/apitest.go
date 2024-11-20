package run

import (
	"os"

	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"

	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
	"github.com/spf13/cobra"
)

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
	creds := regio.Credentials()

	apitestingClient := http.NewAPITester(regio.APIBaseURL(), creds.Username, creds.AccessKey, apitestingTimeout)
	restoClient := http.NewResto(regio, creds.Username, creds.AccessKey, 0)

	r := apitest.Runner{
		Project: p,
		Client:  &apitestingClient,
		Region:  regio,
		Reporters: []report.Reporter{
			&table.Reporter{
				Dst: os.Stdout,
			},
		},
		Async:         gFlags.async,
		TunnelService: &restoClient,
	}

	if err := r.ResolveHookIDs(); err != nil {
		return 1, err
	}

	tracker := usage.DefaultClient
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Framework("apit", ""),
		)
		_ = tracker.Close()
	}()

	log.Info().
		Str("region", regio.String()).
		Str("tunnel", r.Project.Sauce.Tunnel.Name).
		Msg("Running API Test in Sauce Labs.")
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
