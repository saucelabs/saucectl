package apit

import (
	"context"
	"fmt"

	"github.com/saucelabs/saucectl/internal/apitest"
	"github.com/spf13/cobra"
)

func VaultCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "vault",
		Short:        "Commands for interacting with API Testing project vaults",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		SetVariableCommand(),
		GetVariableCommand(),
		GetSnippetCommand(),
	)
	return cmd
}

func resolve(projectName string) (apitest.Hook, error) {
	projects, err := apitesterClient.GetProjects(context.Background())
	if err != nil {
		// log.Error().Err(err).Msg(msg.ProjectListFailure)
		return apitest.Hook{}, err
	}
	var project apitest.ProjectMeta
	for _, p := range projects {
		if p.Name == projectName {
			project = p
			break
		}
	}

	hooks, err := apitesterClient.GetHooks(context.Background(), project.ID)
	if err != nil {
		return apitest.Hook{}, err
	}
	if len(hooks) == 0 {
		return apitest.Hook{}, fmt.Errorf("Project named %s has no hooks configured", projectName)
	}

	return hooks[0], nil
}
