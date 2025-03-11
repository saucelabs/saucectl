package builds

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/build"
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
	buildsService build.Service
	jobService    insights.Service
	userService   iam.UserService
	buildsTimeout = 1 * time.Minute
	iamTimeout    = 1 * time.Minute
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "builds",
		Short:            "Interact with builds",
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
			iamClient := http.NewUserService(url, creds, iamTimeout)
			buildsClient := http.NewBuildService(reg, creds.Username, creds.AccessKey, buildsTimeout)

			userService = &iamClient
			buildsService = &buildsClient

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		//GetCommand(),
		ListCommand(),
	)

	return cmd
}
