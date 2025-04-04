package apit

import (
	"encoding/json"
	"fmt"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			vault, err := apitesterClient.GetVault(cmd.Context(), selectedProject.Hooks[0].Identifier)
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
