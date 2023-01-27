package run

import (
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
	"os"
)

func runImageRunner(cmd *cobra.Command) (int, error) {
	p, err := imagerunner.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	imageRunnerClient.URL = regio.APIBaseURL()

	r := saucecloud.ImgRunner{
		Project:       p,
		RunnerService: &imageRunnerClient,
		Reporters: []report.Reporter{&table.Reporter{
			Dst: os.Stdout,
		}},
	}
	return r.RunProject()
}
