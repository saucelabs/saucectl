package jobs

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/usage"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/cobra"
)

var (
	jobService      insights.Service
	userService     iam.UserService
	insightsTimeout = 1 * time.Minute
	iamTimeout      = 1 * time.Minute
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "jobs",
		Short:            "Interact with jobs",
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
			insightsClient := http.NewInsightsService(url, creds, insightsTimeout)
			iamClient := http.NewUserService(url, creds, iamTimeout)

			jobService = &insightsClient
			userService = &iamClient

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		GetCommand(),
		ListCommand(),
	)

	return cmd
}
