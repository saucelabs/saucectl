package storage

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
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
			// TODO progress bar would be great. Possible?
			resp, err := appsClient.Upload(args[0])
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}

			switch out {
			case "text":
				println(resp.ID)
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
