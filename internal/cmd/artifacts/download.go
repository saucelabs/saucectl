package artifacts

import (
	"errors"
	"fmt"
	"os"
	"path"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/fpath"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func DownloadCommand() *cobra.Command {
	var targetDir string
	var out string
	var hto bool

	cmd := &cobra.Command{
		Use:   "download <jobID/runID> <filename>",
		Short: "Downloads the specified artifacts from the given job/run. Supports glob pattern.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no job or run ID specified")
			}
			if len(args) == 1 || args[1] == "" {
				return errors.New("no file pattern specified")
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
			ID := args[0]
			filePattern := args[1]
			if hto {
				return htoDownload(ID, filePattern, targetDir, out)
			}
			return download(ID, filePattern, targetDir, out)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&targetDir, "target-dir", "", "Save files to target directory. Defaults to current working directory.")
	flags.StringVarP(&out, "out", "o", "text", "Output format to the console. Options: text, json.")
	flags.BoolVar(&hto, "hto", false, "Download artifacts for image runner.")

	return cmd
}

func htoDownload(runID, filePattern, targetDir, outputFormat string) error {
	files, err := artifactSvc.HtoDownload(runID, filePattern, targetDir)
	if err != nil {
		return err
	}
	return renderResults(files, outputFormat)
}

func download(ID, filePattern, targetDir, outputFormat string) error {
	lst, err := artifactSvc.List(ID)
	if err != nil {
		return err
	}

	files := fpath.MatchFiles(lst.Items, []string{filePattern})
	lst.Items = files

	bar := newDownloadProgressBar(outputFormat, len(files))
	for _, f := range files {
		_ = bar.Add(1)
		body, err := artifactSvc.Download(ID, f)
		if err != nil {
			return fmt.Errorf("failed to get file: %w", err)
		}

		filePath := f
		if targetDir != "" {
			if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create target dir: %w", err)
			}
			filePath = path.Join(targetDir, filePath)
		}

		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		_, err = file.Write(body)
		if err != nil {
			return fmt.Errorf("failed to write to the file: %w", err)
		}
		_ = file.Close()
	}
	bar.Close()

	if err := renderResults(lst.Items, outputFormat); err != nil {
		return err
	}

	return nil
}

func renderResults(lst []string, outputFormat string) error {
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

func newDownloadProgressBar(output string, count int) *progressbar.ProgressBar {
	switch output {
	case "json":
		return progressbar.DefaultSilent(int64(count))
	default:
		return progressbar.Default(int64(count))
	}
}
