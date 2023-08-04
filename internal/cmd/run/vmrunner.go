package run

import (
	"errors"
	"os"

	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
	"github.com/saucelabs/saucectl/internal/vmrunner"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
)

func runVMRunner(cmd *cobra.Command) (int, error) {
	p, err := vmrunner.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	rgn := region.FromString(p.Sauce.Region)
	if rgn == region.USEast4 {
		return 1, errors.New(msg.NoFrameworkSupport)
	}

	webdriverClient.URL = rgn.WebDriverBaseURL()
	testcompClient.URL = rgn.APIBaseURL()
	restoClient.URL = rgn.APIBaseURL()
	appsClient.URL = rgn.APIBaseURL()
	insightsClient.URL = rgn.APIBaseURL()
	iamClient.URL = rgn.APIBaseURL()

	log.Info().Msg("Starting VMRunner in Sauce Labs")
	r := saucecloud.VMRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: &appsClient,
			JobService: saucecloud.JobService{
				VDCStarter:    &webdriverClient,
				RDCStarter:    &rdcClient,
				VDCReader:     &restoClient,
				RDCReader:     &rdcClient,
				VDCWriter:     &testcompClient,
				VDCStopper:    &restoClient,
				VDCDownloader: &restoClient,
			},
			TunnelService:   &restoClient,
			MetadataService: &testcompClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			BuildService:    &restoClient,
			Region:          rgn,
			ShowConsoleLog:  true,
			Reporters: []report.Reporter{&table.Reporter{
				Dst: os.Stdout,
			}},
			Async:    gFlags.async,
			FailFast: gFlags.failFast,
		},
	}

	return r.RunProject()
}
