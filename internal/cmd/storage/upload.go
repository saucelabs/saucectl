package storage

import (
	"errors"
	"fmt"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/hashio"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/storage"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func UploadCommand() *cobra.Command {
	var out string
	var force bool
	var description string

	cmd := &cobra.Command{
		Use: "upload filename",
		Short: "Uploads an app file to Sauce Storage and returns a unique file ID assigned to the app. " +
			"Sauce Storage supports app files in *.apk, *.aab, *.ipa, or *.zip format.",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no filename specified")
			}

			return nil
		},
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
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			finfo, err := file.Stat()
			if err != nil {
				return fmt.Errorf("failed to inspect file: %w", err)
			}

			hash, err := hashio.SHA256(args[0])
			if err != nil {
				return fmt.Errorf("failed to compute checksum: %w", err)
			}

			var item storage.Item
			skipUpload := false

			// Look up the file first.
			if !force {
				list, err := appsClient.List(
					cmd.Context(),
					storage.ListOptions{
						SHA256:     hash,
						MaxResults: 1,
					})
				if err != nil {
					return fmt.Errorf("storage lookup failed: %w", err)
				}
				if len(list.Items) > 0 {
					item = list.Items[0]
					skipUpload = true
				}
			}

			// Upload the file if necessary.
			if !skipUpload {
				bar := newProgressBar(out, finfo.Size(), "Uploading")
				reader := progress.NewReadSeeker(file, bar)

				item, err = appsClient.UploadStream(
					cmd.Context(),
					storage.FileInfo{
						Name:        finfo.Name(),
						Description: description,
					},
					&reader,
				)
				if err != nil {
					return fmt.Errorf("failed to upload file: %w", err)
				}
			}

			switch out {
			case "text":
				if skipUpload {
					println("File already stored! The ID of your file is " + item.ID)
					return nil
				}
				println("Success! The ID of your file is " + item.ID)
			case "json":
				if err := renderJSON(item); err != nil {
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
	flags.BoolVar(&force, "force", false,
		"Forces the upload to happen, even if there's already a file in storage with a matching checksum.",
	)
	flags.StringVarP(&description, "description", "d", "", "A description to distinguish your app.")

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
