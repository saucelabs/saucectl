package artifacts

import (
	"errors"
	http2 "github.com/saucelabs/saucectl/internal/http"
	"time"

	"github.com/saucelabs/saucectl/internal/artifacts"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
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
			restoClient := http2.NewResto("", creds.Username, creds.AccessKey, restoTimeout)
			restoClient.URL = url
			rdcClient := rdc.New("", creds.Username, creds.AccessKey, rdcTimeout, config.ArtifactDownload{})
			rdcClient.URL = url
			testcompClient := http2.NewTestComposer("", creds, testComposerTimeout)

			artifactSvc = saucecloud.NewArtifactService(restoClient, rdcClient, testcompClient)

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
