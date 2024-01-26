package imagerunner

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/spf13/cobra"
)

var (
	imagerunnerClient http.ImageRunner
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:          "imagerunner",
		Short:        "Commands for interacting with imagerunner runs",
		SilenceUsage: true,
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
			url := region.FromString(regio).APIBaseURL()

			asyncEventManager, err := imagerunner.NewAsyncEventManager()
			if err != nil {
				return err
			}

			imagerunnerClient = http.NewImageRunner(url, creds, 15*time.Minute, asyncEventManager)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		LogsCommand(),
		ArtifactsCommand(),
	)
	return cmd
}
