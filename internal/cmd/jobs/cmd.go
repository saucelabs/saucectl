package jobs

import (
	"errors"
	http2 "github.com/saucelabs/saucectl/internal/http"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/cmd/jobs/job"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/insights"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
)

var (
	jobSvc          job.Reader
	insightsTimeout = 1 * time.Minute
	iamTimeout      = 1 * time.Minute
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "jobs",
		Short:            "Interact with jobs",
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
			insightsClient := insights.New(url, creds, insightsTimeout)
			iamClient := http2.UserService{
				HTTPClient:  &http.Client{Timeout: iamTimeout},
				URL:         url,
				Credentials: creds,
			}

			jobSvc = saucecloud.JobCommandService{
				Reader:      &insightsClient,
				UserService: &iamClient,
			}

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
