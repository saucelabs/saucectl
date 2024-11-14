package imagerunner

import (
	"context"
	"errors"
	"fmt"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	imgrunner "github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Flags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			return exec(args[0], liveLogs)
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolVarP(&liveLogs, "live", "", false, "Tail the live log output from a running Sauce Orchestrate container.")

	return cmd
}

func exec(runID string, liveLogs bool) error {
	if liveLogs {
		err := imagerunnerClient.GetLiveLogs(context.Background(), runID)
		if err != nil {
			if errors.Is(err, imgrunner.ErrResourceNotFound) {
				return fmt.Errorf("could not find log URL for run with ID (%s): %w", runID, err)
			}
		}
		return err
	}

	log, err := imagerunnerClient.GetLogs(context.Background(), runID)
	if err != nil {
		if errors.Is(err, imgrunner.ErrResourceNotFound) {
			return fmt.Errorf("could not find log URL for run with ID (%s): %w", runID, err)
		}
		return err
	}
	fmt.Println(log)

	return nil
}
