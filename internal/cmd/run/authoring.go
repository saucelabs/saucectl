package run

import (
	"github.com/rs/zerolog/log"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"

	"github.com/saucelabs/saucectl/internal/authoringrun"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
)

func runAuthoring(cmd *cobra.Command, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := authoringrun.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	if gFlags.selectedSuite != "" {
		if err := authoringrun.FilterSuites(&p, gFlags.selectedSuite); err != nil {
			return 1, err
		}
	}

	authoringrun.SetDefaults(&p)

	if err := authoringrun.Validate(p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	creds := regio.Credentials()

	authoringClient := http.NewAuthoringService(regio.APIBaseURL(), creds.Username, creds.AccessKey, testComposerTimeout)
	restoClient := http.NewResto(regio, creds.Username, creds.AccessKey, 0)
	rdcClient := http.NewRDCService(regio, creds.Username, creds.AccessKey, rdcTimeout)

	jobService := saucecloud.JobService{
		RDC:   rdcClient,
		Resto: restoClient,
	}

	r := authoringrun.Runner{
		Project:    p,
		Client:     authoringClient,
		JobService: jobService,
		Region:     regio,
		Reporters:  createReporters(p.Reporters, gFlags.async),
		Async:      gFlags.async,
		FailFast:   gFlags.failFast,
	}

	tracker := usage.DefaultClient
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Framework("authoring", ""),
		)
		_ = tracker.Close()
	}()

	log.Info().
		Str("region", regio.String()).
		Msg("Running authored tests in Sauce Labs.")
	return r.RunProject(cmd.Context())
}
