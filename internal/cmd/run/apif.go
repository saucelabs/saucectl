package run

import (
	"os"

	"github.com/saucelabs/saucectl/internal/apif"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
)

func runApif() (int, error) {
	p, err := apif.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	apif.SetDefaults(&p)
	// TODO: Validate?

	regio := region.FromString(p.Sauce.Region)

	// testcompClient.URL = regio.APIBaseURL()
	// webdriverClient.URL = regio.WebDriverBaseURL()
	// appsClient.URL = regio.APIBaseURL()
	// rdcClient.URL = regio.APIBaseURL()
	// insightsClient.URL = regio.APIBaseURL()
	// iamClient.URL = regio.APIBaseURL()

	restoClient.URL = regio.APIBaseURL()
	apifClient.URL = regio.APIBaseURL()

	r := apif.ApifRunner{
		Project: p,
		Client:  apifClient,
		Region:  regio,
		Reporters: []report.Reporter{
			&table.Reporter{
				Dst: os.Stdout,
			},
		},
		Async: gFlags.async,
		TunnelService: &restoClient,
	}

	return r.RunProject()
}
