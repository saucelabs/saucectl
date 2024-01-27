package imagerunner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func ListCommand() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "Returns the list of containers",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Properties{}.SetFlags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return list(out)
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")

	return cmd
}

func list(outputFormat string) error {
	containers, err := imagerunnerClient.ListContainers(context.Background())
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
	println(t.Render())
}
