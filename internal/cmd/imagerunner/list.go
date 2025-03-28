package imagerunner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func ListCommand() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "Returns the list of containers",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			tracker := usage.DefaultClient

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return list(cmd.Context(), out)
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")

	return cmd
}

func list(ctx context.Context, outputFormat string) error {
	containers, err := imagerunnerClient.ListContainers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container list: %v", err)
	}

	switch outputFormat {
	case "json":
		if err := renderJSON(containers); err != nil {
			return fmt.Errorf("failed to render output: %v", err)
		}
	case "text":
		renderContainersTable(containers)
	default:
		return errors.New("unknown output format")
	}
	return nil
}

func renderContainersTable(containers http.ContainersResp) {
	if len(containers.Items) == 0 {
		println("Cannot find any containers")
		return
	}
	t := table.NewWriter()
	t.SetStyle(defaultTableStyle)
	t.SuppressEmptyColumns()
	t.AppendHeader(table.Row{
		"ID", "Image", "Status", "CreationTime", "TerminationTime",
	})

	for _, c := range containers.Items {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			c.ID,
			c.Image,
			c.Status,
			time.Unix(c.CreationTime, 0).Format(time.RFC3339),
			time.Unix(c.TerminationTime, 0).Format(time.RFC3339),
		})
	}
	fmt.Println(t.Render())
}
