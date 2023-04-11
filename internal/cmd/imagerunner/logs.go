package imagerunner

import (
	"context"
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
	cmd := &cobra.Command{
		Use:   "logs <runID>",
		Short: "Fetch the logs for an imagerunner run",
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
			return exec(args[0])
		},
	}

	return cmd
}

func exec(runID string) error {
	log, err := imagerunnerClient.GetLogs(context.Background(), runID)
	if err != nil {
		if err == imgrunner.ErrResourceNotFound {
			return fmt.Errorf("could not find log URL for run with ID (%s): %w", runID, err)
		}
		return err
	}
	fmt.Println(log)
	return nil
}
