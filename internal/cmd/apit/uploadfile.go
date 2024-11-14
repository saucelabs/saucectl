package apit

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
)

func UploadFileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload-file FILENAME [--project PROJECT_NAME]",
		Short: "Upload a file in vault",
		Long: `Upload a file in a project's vault.

Use [--project] to specify the project by its name or run without [--project] to choose from a list of projects.
`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
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
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			fd, err := os.Open(name)
			if err != nil {
				return err
			}
			_, err = apitesterClient.PutVaultFile(context.Background(), selectedProject.ID, name, fd)
			if err != nil {
				return err
			}
			fmt.Printf("File %q has been successfully stored.\n", name)
			return nil
		},
	}
	return cmd
}
