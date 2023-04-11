package storage

import (
	"errors"
	"fmt"
	"io"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func DownloadCommand() *cobra.Command {
	var filename string

	cmd := &cobra.Command{
		Use:   "download fileID",
		Short: "Downloads an app file from Sauce Storage.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no ID specified")
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
			reader, size, err := appsClient.Download(args[0])
			if err != nil {
				return fmt.Errorf("download failed: %w", err)
			}
			defer reader.Close()

			file, err := os.Create(filename)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			defer file.Close()

			bar := progressbar.DefaultBytes(size, "Downloading")
			_, err = io.Copy(io.MultiWriter(file, bar), reader)
			_ = bar.Close()
			if err != nil {
				return fmt.Errorf("failed to write to file: %w", err)
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&filename, "filename", "f", "",
		"Save the file to disk with this name.",
	)

	_ = cmd.MarkFlagRequired("filename")

	return cmd
}
