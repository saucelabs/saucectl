package apit

import (
	"errors"
	"fmt"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
)

func GetVariableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-variable NAME [--project PROJECT_NAME]",
		Short: "Get a vault variable",
		Long: `Get a variable value from a project's vault. 
	
Use [--project] to specify the project by its name or run without [--project] to choose from a list of projects.`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no variable name specified")
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
			name := args[0]

			vault, err := apitesterClient.GetVault(cmd.Context(), selectedProject.Hooks[0].Identifier)
			if err != nil {
				return err
			}

			for _, v := range vault.Variables {
				if v.Name == name {
					fmt.Println(v.Value)
					return nil
				}
			}

			return fmt.Errorf("project %q has no vault variable with name %q", selectedProject.ProjectMeta.Name, name)
		},
	}

	return cmd
}
