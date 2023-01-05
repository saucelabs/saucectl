package run

import (
	"os"

	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
)

func runApitest() (int, error) {
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
		Client:  apitestingClient,
		Region:  regio,
		Reporters: []report.Reporter{
			&table.Reporter{
				Dst: os.Stdout,
			},
		},
		Async:         gFlags.async,
		TunnelService: &restoClient,
	}

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
