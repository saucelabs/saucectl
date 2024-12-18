package imagerunner

import (
	"context"
	"errors"
	"fmt"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	imgrunner "github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func LogsCommand() *cobra.Command {
	var liveLogs bool

	cmd := &cobra.Command{
		Use:          "logs <runID>",
		Short:        "Fetch the logs for an imagerunner run",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no run ID specified")
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			tracker := usage.DefaultClient

			go func() {
				tracker.Collect(
					cmds.FullName(cmd),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return exec(cmd.Context(), args[0], liveLogs)
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolVarP(&liveLogs, "live", "", false, "Tail the live log output from a running Sauce Orchestrate container.")

	return cmd
}

func exec(ctx context.Context, runID string, liveLogs bool) error {
	if liveLogs {
		err := imagerunnerClient.GetLiveLogs(ctx, runID)
		if err != nil {
			if errors.Is(err, imgrunner.ErrResourceNotFound) {
				return fmt.Errorf("could not find log URL for run with ID (%s): %w", runID, err)
			}
		}
		return err
	}

	log, err := imagerunnerClient.GetLogs(ctx, runID)
	if err != nil {
		if errors.Is(err, imgrunner.ErrResourceNotFound) {
			return fmt.Errorf("could not find log URL for run with ID (%s): %w", runID, err)
		}
		return err
	}
	fmt.Println(log)

	return nil
}
