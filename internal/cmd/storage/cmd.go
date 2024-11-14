package storage

import (
	"errors"
	"time"

	"github.com/saucelabs/saucectl/internal/usage"

	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/cobra"
)

var (
	appsClient http.AppStore
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "storage",
		Short:            "Interact with Sauce Storage",
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

			appsClient = *http.NewAppStore(reg.APIBaseURL(),
				credentials.Get().Username, credentials.Get().AccessKey,
				15*time.Minute)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		ListCommand(),
		UploadCommand(),
		DownloadCommand(),
		DeleteCommand(),
	)

	return cmd
}
