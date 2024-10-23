package artifacts

import (
	"errors"
	"fmt"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
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
		Use:          "upload <jobID> <filename>",
		Short:        "Uploads an artifact for the job.",
		Long:         "Uploads an artifact for the job. Real Device job is not supported.",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no job ID specified")
			}
			if len(args) == 1 || args[1] == "" {
				return errors.New("no file name specified")
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
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Properties{}.SetFlags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]
			filePath := args[1]
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

			err = artifactSvc.Upload(jobID, finfo.Name(), content)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}
			bar.Close()

			switch out {
			case "text":
				fmt.Println("Success!")
			case "json":
				if err := renderJSON(filePath); err != nil {
					return fmt.Errorf("failed to render output: %w", err)
				}
			default:
				return errors.New("unknown output format")
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", "text", "Output out to the console. Options: text, json.")

	return cmd
}

func newProgressBar(outputFormat string, size int64, description ...string) *progressbar.ProgressBar {
	switch outputFormat {
	case "text":
		return progressbar.DefaultBytes(size, description...)
	default:
		return progressbar.DefaultBytesSilent(size, description...)
	}
}
