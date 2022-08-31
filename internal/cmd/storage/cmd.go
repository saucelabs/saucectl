package storage

import (
	"errors"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/spf13/cobra"
	"net/http"
	"time"
)

var (
	appsClient appstore.AppStore
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "storage",
		Short:            "Interact with the Sauce Storage.",
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			if region.FromString(regio) == region.None {
				return errors.New("invalid region")
			}

			appsClient = appstore.AppStore{
				HTTPClient: &http.Client{Timeout: 15 * time.Minute},
				URL:        region.FromString(regio).APIBaseURL(),
				Username:   credentials.Get().Username,
				AccessKey:  credentials.Get().AccessKey,
			}

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		ListCommand(),
		UploadCommand(),
		DownloadCommand(),
	)

	return cmd
}
