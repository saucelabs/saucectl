package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/cmd/jobs/job"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func GetCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get job by id",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no job ID specified")
			}
			return nil
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Properties{}.SetFlags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if out != JSONOutput && out != TextOutput {
				return errors.New("unknown output format")
			}
			return get(args[0], out)
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")

	return cmd
}

func get(jobID, outputFormat string) error {
	job, err := jobSvc.ReadJob(context.Background(), jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	switch outputFormat {
	case "json":
		if err := renderJSON(job); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderJobTable(job)
	}

	return nil
}

func renderJobTable(job job.Job) {
	t := table.NewWriter()
	t.SetStyle(defaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"ID", "Name", "Source", "Status", "Platform", "Framework", "Browser", "Device",
	})

	// the order of values must match the order of the header
	t.AppendRow(table.Row{
		job.ID,
		job.Name,
		job.Source,
		job.Status,
		job.Platform,
		job.Framework,
		job.BrowserName,
		job.Device,
	})
	t.SuppressEmptyColumns()

	println(t.Render())
}
