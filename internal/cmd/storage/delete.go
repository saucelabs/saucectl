package storage

import (
	"errors"
	"fmt"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func DeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <fileID>",
		Short:        "Delete a file from Sauce Storage.",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no ID specified")
			}

			return nil
		},
		PreRun: func(cmd *cobra.Command, _ []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := appsClient.Delete(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete file: %v", err)
			}

			fmt.Println("File deleted successfully!")

			return nil
		},
	}

	return cmd
}
