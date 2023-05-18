package apit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/saucelabs/saucectl/internal/http"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [--project PROJECT_NAME]",
		Short: "Get the entire vault contents",
		Long: `Use [--project] to specify the project by its name or run with [--project] to 
choose from a list of projects.
`,
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}

			// tracker := segment.DefaultTracker

			// go func() {
			// 	tracker.Collect(
			// 		cases.Title(language.English).String(cmds.FullName(cmd)),
			// 		usage.Properties{}.SetFlags(cmd.Flags()),
			// 	)
			// 	_ = tracker.Close()
			// }()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			vault, err := apitesterClient.GetVault(context.Background(), selectedProject.Hooks[0].Identifier)
			if err != nil {
				return err
			}

			json.NewEncoder(os.Stdout).Encode(vault)

			return nil
		},
	}
	return cmd
}
