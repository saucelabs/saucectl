package apit

import (
	"context"
	"errors"
	"fmt"

	"github.com/saucelabs/saucectl/internal/http"
	"github.com/spf13/cobra"
)

func GetVariableCommand() *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:          "get-variable <variableName>",
		Short:        "Get a vault variable",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no variable name specified")
			}
			return nil
		},
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
			return getVariable(project, args[0])
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "The name of the project the vault belongs to.")

	return cmd
}

func getVariable(projectName string, name string) error {
	project, err := resolve(projectName)
	if err != nil {
		return err
	}

	vault, err := apitesterClient.GetVault(context.Background(), project.Hooks[0].Identifier)
	if err != nil {
		return err
	}

	for _, v := range vault.Variables {
		if v.Name == name {
			fmt.Printf("%s=%s\n", v.Name, v.Value)
			return nil
		}
	}

	return fmt.Errorf("Project (%s) has no vault variable with name (%s)", project.ProjectMeta.Name, name)
}
