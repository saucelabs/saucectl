package apit

import (
	"context"
	"errors"
	"fmt"

	"github.com/saucelabs/saucectl/internal/http"
	"github.com/spf13/cobra"
)

func GetVariableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get-variable <projectName> <name>",
		Short:        "Get a vault variable",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (args[0] == "" || args[1] == "") {
				// TODO: Give useful error message
				return errors.New("no project name specified")
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
			return getVariable(args[0], args[1])
		},
	}

	return cmd
}

func getVariable(projectName string, name string) error {
	hook, err := resolve(projectName)	
	if err != nil {
		return err
	}

	vault, err := apitesterClient.GetVault(context.Background(), hook.Identifier)
	if err != nil {
		return err
	}

	for _, v := range vault.Variables {
		if v.Name == name {
			fmt.Printf("%s=%s", v.Name, v.Value)
			return nil
		}
	}

	return fmt.Errorf("Project (%s) has no vault variabled with name (%s)", projectName, name)
}
