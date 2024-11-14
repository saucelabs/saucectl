package artifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
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
		DrawBorder:      true,
		SeparateColumns: false,
		SeparateFooter:  true,
		SeparateHeader:  true,
		SeparateRows:    false,
	},
	Title: table.TitleOptionsDefault,
}

// List represents artifact structure
type List struct {
	JobID string   `json:"jobID"`
	Items []string `json:"items"`
}

func ListCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use: "list <jobID>",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of artifacts for the specified job.",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no job ID specified")
			}

			return nil
		},
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			tracker := segment.DefaultClient

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			return list(args[0], out)
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")

	return cmd
}

func list(jobID, outputFormat string) error {
	lst, err := artifactSvc.List(jobID)
	if err != nil {
		return fmt.Errorf("failed to get artifacts list: %w", err)
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

func renderTable(lst List) {
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

	fmt.Println(t.Render())
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}
