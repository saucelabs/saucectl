package artifacts

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/artifacts"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
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

	cmd := &cobra.Command{
		Use:              "artifacts",
		Short:            "Interact with job artifacts",
		SilenceUsage:     true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			reg := region.FromString(regio)
			if reg == region.None {
				return errors.New("invalid region")
			}
			if reg == region.Staging {
				segment.DefaultTracker.Enabled = false
			}

			creds := credentials.Get()
			url := reg.APIBaseURL()
			restoClient := http.NewResto(url, creds.Username, creds.AccessKey, restoTimeout)
			rdcClient := http.NewRDCService(url, creds.Username, creds.AccessKey, rdcTimeout, config.ArtifactDownload{})
			testcompClient := http.NewTestComposer(url, creds, testComposerTimeout)

			artifactSvc = saucecloud.NewArtifactService(&restoClient, &rdcClient, &testcompClient)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		DownloadCommand(),
		ListCommand(),
		UploadCommand(),
	)

	return cmd
}
