package apit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/saucelabs/saucectl/internal/http"
	"github.com/spf13/cobra"
)

func GetVaultCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get-vault <projectName>",
		Short:        "Get a project's vault",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
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
			return exec(args[0])
		},
	}

	return cmd
}

func exec(projectName string) error {
	projects, err := apitesterClient.GetProjects(context.Background());
	if err != nil {
		return err
	}

	var projectID string
	for _, p := range projects {
		if p.Name == projectName {
			projectID = p.ID
		}
	}

	if projectID == "" {
		return fmt.Errorf("Can't find project with name %s", projectName)
	}

	hooks, err := apitesterClient.GetHooks(context.Background(), projectID);
	if err != nil {
		return err
	}
	if len(hooks) == 0 {
		return fmt.Errorf("Project has no hooks")
	}

	vault, err := apitesterClient.GetVault(context.Background(), hooks[0].Identifier)
	if err != nil {
		return err
	}
	json.NewEncoder(os.Stdout).Encode(vault)
	return nil
}
