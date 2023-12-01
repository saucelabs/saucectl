package docker

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/cobra"
)

var (
	registryClient        http.DockerRegistry
	registryClientTimeout = 1 * time.Minute
	registryPushTimeout   = 1 * time.Minute
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
			registryClient = http.NewDockerRegistry(url, creds.Username, creds.AccessKey, registryClientTimeout)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "login-region", "r", "us-west-1", "The Sauce Labs region to login. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		PushCommand(),
	)

	return cmd
}
