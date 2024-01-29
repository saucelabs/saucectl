package docker

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/cobra"
)

var (
	imageRunnerService        http.ImageRunner
	imageRunnerServiceTimeout = 1 * time.Minute
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "docker",
		Short:            "Interact with Sauce Container Registry",
		SilenceUsage:     true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			reg := region.FromString(regio)
			if reg == region.None {
				return errors.New("invalid region: must be one of [us-west-1, eu-central-1]")
			}

			creds := reg.Credentials()
			url := reg.APIBaseURL()

			asyncEventManager, err := imagerunner.NewAsyncEventMgr()
			if err != nil {
				return err
			}

			imageRunnerService = http.NewImageRunner(url, creds, imageRunnerServiceTimeout, asyncEventManager)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region to login. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		PushCommand(),
	)

	return cmd
}
