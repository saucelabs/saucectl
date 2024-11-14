package apit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [--project PROJECT_NAME]",
		Short: "Get the entire vault contents",
		Long: `Print the vault contents as json to stdout. 

Use [--project] to specify the project by its name or run without [--project] to choose from a list of projects.
`,
		SilenceUsage: true,
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
		RunE: func(_ *cobra.Command, _ []string) error {
			vault, err := apitesterClient.GetVault(context.Background(), selectedProject.Hooks[0].Identifier)
			if err != nil {
				return err
			}

			err = json.NewEncoder(os.Stdout).Encode(vault)
			if err != nil {
				return fmt.Errorf("failed to convert vault to json: %w", err)
			}

			return nil
		},
	}
	return cmd
}
