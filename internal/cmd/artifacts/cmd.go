package artifacts

import (
	"errors"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/artifacts"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
)

var (
	artifactSvc         artifacts.Service
	rdcTimeout          = 1 * time.Minute
	restoTimeout        = 1 * time.Minute
	testComposerTimeout = 1 * time.Minute
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string
	var isRDC bool
	var jobID string

	cmd := &cobra.Command{
		Use:              "artifacts",
		Short:            "Interact with job artifacts",
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			if region.FromString(regio) == region.None {
				return errors.New("invalid region")
			}

			creds := credentials.Get()
			url := region.FromString(regio).APIBaseURL()
			restoClient := resto.New("", creds.Username, creds.AccessKey, restoTimeout)
			restoClient.URL = url
			rdcClient := rdc.New("", creds.Username, creds.AccessKey, rdcTimeout, config.ArtifactDownload{})
			rdcClient.URL = url
			testcompClient := testcomposer.Client{
				HTTPClient:  &http.Client{Timeout: testComposerTimeout},
				URL:         url,
				Credentials: creds,
			}

			artifactSvc = saucecloud.NewArtifactService(restoClient, rdcClient, testcompClient, jobID, isRDC)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")
	flags.BoolVar(&isRDC, "rdc", false, "Get RDC job details")
	flags.StringVarP(&jobID, "job", "j", "", "Target job id")

	_ = cmd.MarkFlagRequired("job")

	cmd.AddCommand(
		DownloadCommand(),
		ListCommand(),
		UploadCommand(),
	)

	return cmd
}
