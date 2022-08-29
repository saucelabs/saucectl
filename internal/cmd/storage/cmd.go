package storage

import (
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

func Command() *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "storage",
		Short:            "Interact with the Sauce Storage.",
		TraverseChildren: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			appsClient = appstore.AppStore{
				HTTPClient: &http.Client{Timeout: 10 * time.Second},
				URL:        region.FromString(regio).APIBaseURL(),
				Username:   credentials.Get().Username,
				AccessKey:  credentials.Get().AccessKey,
			}
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region.")

	cmd.AddCommand(
		ListCommand(),
		UploadCommand(),
		DownloadCommand(),
	)

	return cmd
}
