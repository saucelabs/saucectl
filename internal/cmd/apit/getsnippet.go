package apit

import (
	"context"
	"errors"
	"fmt"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func GetSnippetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-snippet NAME [--project PROJECT_NAME]",
		Short: "Get a vault snippet",
		Long: `Get a snippet from a project's vault. 

Use [--project] to specify the project by its name or run without [--project] to choose from a list of projects.
`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no snippet name specified")
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
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			vault, err := apitesterClient.GetVault(context.Background(), selectedProject.Hooks[0].Identifier)
			if err != nil {
				return err
			}

			v, ok := vault.Snippets[name]
			if !ok {
				return fmt.Errorf("project %q has no vault snippet with name %q", selectedProject.ProjectMeta.Name, name)
			}

			fmt.Printf("%s", v)
			return nil
		},
	}
	return cmd
}
