package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/human"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	var query string
	var name string
	var out string
	var sha256 string

	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of files that have been uploaded to Sauce Storage.",
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Properties{}.SetFlags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := appsClient.List(
				cmd.Context(),
				storage.ListOptions{
					Q:      query,
					Name:   name,
					SHA256: sha256,
				})
			if err != nil {
				return fmt.Errorf("failed to retrieve list: %w", err)
			}
			switch out {
			case "text":
				renderTable(list)
			case "json":
				if err := renderJSON(list); err != nil {
					return fmt.Errorf("failed to render output: %w", err)
				}
			default:
				return errors.New("unknown output format")
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, "query", "q", "",
		"Any search term (such as app name, file name, description, build number or version) by which you want to filter.",
	)
	flags.StringVarP(&name, "name", "n", "",
		"The filename (case-insensitive) by which you want to filter.",
	)
	flags.StringVar(&sha256, "sha256", "",
		"The checksum of the file by which you want to filter.",
	)
	flags.StringVarP(&out, "out", "o", "text",
		"Output format to the console. Options: text, json.",
	)

	return cmd
}

func renderTable(list storage.List) {
	if len(list.Items) == 0 {
		println("No files match the search criteria.")
	}

	t := table.NewWriter()
	t.SetStyle(defaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{"Size", "Uploaded", "ID", "Name"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Name:        "Size",
			AlignHeader: text.AlignLeft,
			Align:       text.AlignRight,
			AlignFooter: text.AlignRight,
			Transformer: func(val interface{}) string {
				t, _ := val.(int)
				return human.Bytes(int64(t))
			},
		},
		{
			Name:        "Uploaded",
			Align:       text.AlignRight,
			AlignFooter: text.AlignRight,
			Transformer: func(val interface{}) string {
				t, _ := val.(time.Time)
				return t.Format(time.Stamp)
			},
		},
		{
			Name: "ID",
		},
		{
			Name: "Name",
		},
	})

	for _, item := range list.Items {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{item.Size, item.Uploaded, item.ID, item.Name})
	}

	fmt.Println(t.Render())

	if list.Truncated {
		println("\nYour query returned more files than we can display. Please refine your query.")
	}
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}
