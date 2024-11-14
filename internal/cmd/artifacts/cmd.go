package artifacts

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/usage"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
)

var (
	artifactSvc         ArtifactService
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
				usage.DefaultClient.Enabled = false
			}

			creds := credentials.Get()
			url := reg.APIBaseURL()

			artifactSvc = ArtifactService{
				JobService: saucecloud.JobService{
					Resto: http.NewResto(
						reg, creds.Username, creds.AccessKey, restoTimeout,
					),
					RDC: http.NewRDCService(
						reg, creds.Username, creds.AccessKey, rdcTimeout,
					),
					TestComposer: http.NewTestComposer(
						url, creds, testComposerTimeout,
					),
				},
			}

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
