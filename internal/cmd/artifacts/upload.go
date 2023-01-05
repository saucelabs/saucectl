package artifacts

import (
	"errors"
	"fmt"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func UploadCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Uploads an artifacts for the job",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no filename specified")
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
			filePath := args[0]
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file [%s]: %w", filePath, err)
			}
			finfo, err := file.Stat()
			if err != nil {
				return fmt.Errorf("failed to inspect file: %w", err)
			}

			bar := newProgressBar(out, finfo.Size(), "Uploading")
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			err = artifactSvc.Upload(finfo.Name(), content)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}
			bar.Close()

			switch out {
			case "text":
				println("Success!")
			case "json":
				if err := renderJSON(filePath); err != nil {
					return fmt.Errorf("failed to render output: %w", err)
				}
			default:
				return errors.New("unknown output out")
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", "text", "Output out to the console. Options: text, json.")

	return cmd
}

func newProgressBar(outputout string, size int64, description ...string) *progressbar.ProgressBar {
	switch outputout {
	case "text":
		return progressbar.DefaultBytes(size, description...)
	default:
		return progressbar.DefaultBytesSilent(size, description...)
	}
}
