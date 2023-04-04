package artifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/saucelabs/saucectl/internal/artifacts"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
)

var (
	artifactSvc         artifacts.Service
	rdcTimeout          = 1 * time.Minute
	restoTimeout        = 1 * time.Minute
	testComposerTimeout = 1 * time.Minute
	runnerSvcTimeout    = 1 * time.Minute
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	var regio string

	cmd := &cobra.Command{
		Use:              "artifacts",
		Short:            "Interact with job artifacts",
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
			restoClient := http.NewResto(url, creds.Username, creds.AccessKey, restoTimeout)
			rdcClient := http.NewRDCService(url, creds.Username, creds.AccessKey, rdcTimeout, config.ArtifactDownload{})
			testcompClient := http.NewTestComposer(url, creds, testComposerTimeout)

			artifactSvc = saucecloud.NewArtifactService(&restoClient, &rdcClient, &testcompClient)

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&regio, "region", "r", "us-west-1", "The Sauce Labs region. Options: us-west-1, eu-central-1.")

	cmd.AddCommand(
		DownloadCommand(),
		ListCommand(),
		UploadCommand(),
	)

	return cmd
}

func renderResults(lst artifacts.List, outputFormat string) error {
	switch outputFormat {
	case "json":
		if err := renderJSON(lst); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderTable(lst)
	default:
		return errors.New("unknown output format")
	}

	return nil
}

func renderTable(lst artifacts.List) {
	if len(lst.Items) == 0 {
		println("No artifacts for this job.")
		return
	}

	t := table.NewWriter()
	t.SetStyle(defaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{"Items"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Name: "Items",
		},
	})

	for _, item := range lst.Items {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{item})
	}
	t.SuppressEmptyColumns()

	println(t.Render())
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}
