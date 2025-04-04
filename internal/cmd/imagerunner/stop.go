package imagerunner

import (
	"errors"
	"fmt"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func StopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "stop <runID>",
		Short:        "Stop the running container",
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
			ID := args[0]
			fmt.Printf("Stopping container %s...\n", ID)
			if err := imagerunnerClient.StopRun(cmd.Context(), ID); err != nil {
				return fmt.Errorf("failed to stop the container: %v", err)
			}
			return nil
		},
	}

	return cmd
}
