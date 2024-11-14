package imagerunner

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/ryanuber/go-glob"
	szip "github.com/saucelabs/saucectl/internal/archive/zip"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/fileio"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/imagerunner"
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

func ArtifactsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "artifacts",
		Short:        "Commands for interacting with artifacts produced by the imagerunner.",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		downloadCommand(),
	)

	return cmd
}

func downloadCommand() *cobra.Command {
	var targetDir string
	var out string

	cmd := &cobra.Command{
		Use:          "download <runID> <file-pattern>",
		Short:        "Downloads the specified artifacts from the given run. Supports glob pattern.",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no run ID specified")
			}
			if len(args) == 1 || args[1] == "" {
				return errors.New("no file pattern specified")
			}

			return nil
		},
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			tracker := segment.DefaultTracker

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
			ID := args[0]
			filePattern := args[1]

			return download(ID, filePattern, targetDir, out)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&targetDir, "target-dir", "", "Save files to target directory. Defaults to current working directory.")
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")

	return cmd
}

func download(ID, filePattern, targetDir, outputFormat string) error {
	reader, err := imagerunnerClient.DownloadArtifacts(context.Background(), ID)
	if err != nil {
		return fmt.Errorf("failed to fetch artifacts: %w", err)
	}
	defer reader.Close()

	fileName, err := fileio.CreateTemp(reader)
	if err != nil {
		return fmt.Errorf("failed to download artifacts content: %w", err)
	}
	defer os.Remove(fileName)

	zf, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer zf.Close()

	files := []string{}
	for _, f := range zf.File {
		if glob.Glob(filePattern, f.Name) {
			files = append(files, f.Name)
			if err = szip.Extract(targetDir, f); err != nil {
				return fmt.Errorf("failed to extract file: %w", err)
			}
		}
	}

	lst := imagerunner.ArtifactList{
		ID:    ID,
		Items: files,
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

func renderTable(lst imagerunner.ArtifactList) {
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

	fmt.Println(t.Render())
}

func renderJSON(val any) error {
	return json.NewEncoder(os.Stdout).Encode(val)
}
