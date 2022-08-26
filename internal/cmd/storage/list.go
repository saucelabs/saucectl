package storage

import (
	"encoding/json"
	"github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/spf13/cobra"
	"os"
	"time"
)

// TODO maybe expose report.table.defaultTableStyle instead?
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

	cmd := &cobra.Command{
		Use: "list",
		Aliases: []string{
			"ls",
		},
		Short:        "Returns the list of files that have been uploaded to Sauce Storage.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			list, err := appsClient.List(storage.ListOptions{
				Q:    query,
				Name: name,
			})
			if err != nil {
				log.Err(err).Msg("Failed to retrieve files")
			}
			// TODO handle empty list case (not an error, but nothing was found)
			switch out {
			case "text":
				renderTable(list)
			case "json":
				renderJSON(list)
			default:
				log.Error().Msgf("Unknown output format: %s", out)
			}
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, "query", "q", "",
		"Any search term (such as app name, file name, description, build number or version) by which you want to filter.",
	)
	flags.StringVarP(&name, "name", "n", "",
		"The file name (case-insensitive) by which you want to filter.",
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

	t.AppendHeader(table.Row{"Size", "Uploaded", "Name", "ID"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Name:        "Size",
			AlignHeader: text.AlignLeft,
			Align:       text.AlignRight,
			AlignFooter: text.AlignRight,
			Transformer: func(val interface{}) string {
				t, _ := val.(int)
				return humanize.Bytes(uint64(t))
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
			Name: "Name",
		},
		{
			Name: "ID",
		},
	})

	for _, item := range list.Items {
		// the order of values must match the order of the header
		t.AppendRow(table.Row{item.Size, item.Uploaded, item.Name, item.ID})
	}

	println(t.Render())

	if list.Truncated {
		println("\nYour query returned more files than we can display. Please refine your query.")
	}
}

func renderJSON(list storage.List) error {
	return json.NewEncoder(os.Stdout).Encode(list)
}
