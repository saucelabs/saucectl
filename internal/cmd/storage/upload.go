package storage

import (
	"errors"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"os"
)

func UploadCommand() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:   "upload filename",
		Short: "Uploads an app file to Sauce Storage and returns a unique file ID assigned to the app. Sauce Storage supports app files in *.apk, *.aab, *.ipa, or *.zip format.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no filename specified")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			finfo, err := file.Stat()
			if err != nil {
				return fmt.Errorf("failed to inspect file: %w", err)
			}

			bar := newProgressBar(out, finfo.Size(), "Uploading")
			reader := progressbar.NewReader(file, bar)

			resp, err := appsClient.UploadStream(finfo.Name(), &reader)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}

			switch out {
			case "text":
				println("Success! The ID of your file is " + resp.ID)
			case "json":
				if err := renderJSON(resp); err != nil {
					return fmt.Errorf("failed to render output: %w", err)
				}
			default:
				return errors.New("unknown output format")
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&out, "out", "o", "text",
		"Output format to the console. Options: text, json.",
	)

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
