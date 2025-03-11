package builds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/job"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

const (
	JSONOutput = "json"
	TextOutput = "text"
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
	var buildSource string

	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of builds",
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			tracker := usage.DefaultClient

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if page < 0 {
				return errors.New("invalid page")
			}
			if size < 0 {
				return errors.New("invalid size")
			}
			if out != JSONOutput && out != TextOutput {
				return errors.New("unknown output format")
			}
			var isStatusValid bool

			stat := build.Status(status)
			for _, s := range build.AllStates {
				if s == stat {
					isStatusValid = true
					break
				}
			}

			if status != "" && !isStatusValid {
				return fmt.Errorf("unknown status. Options: %s", strings.Join(job.AllStates, ", "))
			}

			src := build.Source(buildSource)
			if src != build.SourceRDC && src != build.SourceVDC {
				return errors.New("invalid job resource. Options: vdc, rdc")
			}

			return list(cmd.Context(), out, page, size, stat, build.Source(buildSource))
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.IntVarP(&page, "page", "p", 0, "Page for pagination. Default is 0.")
	flags.IntVarP(&size, "size", "s", 20, "Per page for pagination. Default is 20.")
	flags.StringVar(&status, "status", "", "Filter job using status. Options: passed, failed, error, complete, in progress, queued.")
	flags.StringVar(&buildSource, "source", "", "Job source from saucelabs. Options: vdc, rdc.")

	return cmd
}

func list(ctx context.Context, format string, page int, size int, status build.Status, source build.Source) error {
	user, err := userService.User(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	opts := build.ListBuildsOptions{
		UserID: user.ID,
		Page:   page,
		Size:   size,
		Status: status,
		Source: source,
	}

	builds, err := buildsService.ListBuilds(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to get builds: %w", err)
	}

	switch format {
	case "json":
		if err := renderJSON(builds); err != nil {
			return fmt.Errorf("failed to render output: %w", err)
		}
	case "text":
		renderListTable(builds)
	}

	return nil
}

func renderListTable(builds []build.Build) {
	if len(builds) == 0 {
		println("Cannot find any builds")
		return
	}

	t := table.NewWriter()
	t.SetStyle(defaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{
		"ID", "Name", "Status",
	})

	for _, item := range builds {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{
			item.ID,
			item.Name,
			item.Status,
		})
	}
	t.SuppressEmptyColumns()
	t.AppendFooter(table.Row{
		fmt.Sprintf("%d builds in total", len(builds)),
	})

	fmt.Println(t.Render())
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}
