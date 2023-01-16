package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	RDC = "rdc"
	VDC = "vdc"
)

var defaultTableStyle = table.Style{
	Name: "saucy",
	Box: table.BoxStyle{
		BottomLeft:       "└",
		BottomRight:      "┘",
		BottomSeparator:  "",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneCount("+")),
		Left:             "│",
		LeftSeparator:    "",
		MiddleHorizontal: "─",
		MiddleSeparator:  "",
		MiddleVertical:   "",
		PaddingLeft:      " ",
		PaddingRight:     " ",
		PageSeparator:    "\n",
		Right:            "│",
		RightSeparator:   "",
		TopLeft:          "┌",
		TopRight:         "┐",
		TopSeparator:     "",
		UnfinishedRow:    " ...",
	},
	Color: table.ColorOptionsDefault,
	Format: table.FormatOptions{
		Footer: text.FormatDefault,
		Header: text.FormatDefault,
		Row:    text.FormatDefault,
	},
	HTML: table.DefaultHTMLOptions,
	Options: table.Options{
		DrawBorder:      false,
		SeparateColumns: false,
		SeparateFooter:  true,
		SeparateHeader:  true,
		SeparateRows:    false,
	},
	Title: table.TitleOptionsDefault,
}

func ListCommand() *cobra.Command {
	var out string
	var page int
	var size int
	var status string
	var jobSource string

	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short: "Returns the list of jobs",
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
			return list(jobSource, out, buildQueryOpts(page, size, status))
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.IntVarP(&page, "page", "p", 0, "Page for pagination. Default is 0.")
	flags.IntVarP(&size, "size", "s", 20, "Per page for pagination. Default is 20.")
	flags.StringVar(&status, "status", "", "Filter job using status. Options: passed, failed, error, complete, in progress, queued.")
	flags.StringVar(&jobSource, "source", "", "Job source from saucelabs. Options: vdc, rdc, api.")

	return cmd
}

func buildQueryOpts(page, size int, status string) job.QueryOption {
	return job.QueryOption{
		Page:   page,
		Size:   size,
		Status: status,
	}
}

func list(jobSource string, outputFormat string, queryOpts job.QueryOption) error {
	lst, err := jobSvc.ListJobs(context.Background(), jobSource, queryOpts)
	if err != nil {
		return fmt.Errorf("failed to get job list: %w", err)
	}

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

func renderTable(lst job.List) {
	if len(lst.Jobs) == 0 {
		println("Cannot find any jobs")
		return
	}

	t := table.NewWriter()
	t.SetStyle(defaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"ID", "Name", "Status", "Platform", "Framework", "Browser", "Device",
	})

	for _, item := range lst.Jobs {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			item.ID,
			item.Name,
			item.Status,
			item.Platform,
			item.Framework,
			item.BrowserName,
			item.Device,
		})
	}
	t.SuppressEmptyColumns()
	t.AppendFooter(table.Row{
		fmt.Sprintf("Page: %d", lst.Page),
		fmt.Sprintf("Size: %d", lst.Size),
		fmt.Sprintf("%d jobs in total", lst.Total),
	})

	println(t.Render())
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}
