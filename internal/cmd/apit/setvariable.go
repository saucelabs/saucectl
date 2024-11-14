package apit

import (
	"context"
	"errors"
	"fmt"

	"github.com/saucelabs/saucectl/internal/apitest"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func SetVariableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-variable NAME VALUE [--project PROJECT_NAME]",
		Short: "Set a vault variable",
		Long: `Set/update a variable in a project's vault. If a variable NAME is already in the vault,
the value will be updated, otherwise a new variable will be added. 

Use [--project] to specify a project by its name or run without [--project] to choose from a list of projects.
`,
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no variable name specified")
			}
			if len(args) == 1 || args[1] == "" {
				return errors.New("no variable value specified")
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
			name := args[0]
			val := args[1]
			updateVault := apitest.Vault{
				Variables: []apitest.VaultVariable{
					{
						Name:  name,
						Value: val,
						Type:  "variable",
					},
				},
				Snippets: map[string]string{},
			}

			err := apitesterClient.PutVault(context.Background(), selectedProject.Hooks[0].Identifier, updateVault)
			if err != nil {
				return err
			}
			return nil
		},
	}

	return cmd
}
