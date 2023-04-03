package hto

import (
	"context"
	"fmt"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func LogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <runID>",
		Short: "Fetch the logs for an HTO run",
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
			return exec(args[0])
		},
	}

	return cmd
}

func exec(runID string) error {
	log, err := imagerunnerClient.GetLogs(context.Background(), runID)
	if err != nil {
		return err
	}
	fmt.Println(log)
	return nil
}
