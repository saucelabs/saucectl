package apit

import (
	"context"
	"errors"
	"fmt"

	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/spf13/cobra"
)

func SetVariableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "set-variable <projectName> <name> <value>",
		Short:        "Set a vault variable",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (args[0] == "" || args[1] == "" || args[2] == "") {
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
			return setVariable(args[0], args[1], args[2])
		},
	}

	return cmd
}

func setVariable(projectName string, name string, val string) error {
	hook, err := resolve(projectName)
	if err != nil {
		return err
	}

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

	err = apitesterClient.PutVault(context.Background(), hook.Identifier, updateVault)
	if err != nil {
		return err
	}
	return nil
}
