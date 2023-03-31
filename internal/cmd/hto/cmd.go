package hto

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/cobra"
)

var (
	imagerunnerClient http.ImageRunner
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:   "hto",
		Short: "Commands for interacting with Hosted Test Orchestration (HTO) runs",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			if region.FromString(regio) == region.None {
				return errors.New("invalid region")
			}

			creds := credentials.Get()
			url := region.FromString(regio).APIBaseURL()

			imagerunnerClient = http.NewImageRunner(url, creds, 15*time.Minute)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		LogsCommand(),
	)
	return cmd
}
