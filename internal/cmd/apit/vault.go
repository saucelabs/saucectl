package apit

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
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
		SetSnippetCommand(),
	)
	return cmd
}

func projectSurvey(names []string) string {
	var selection string
	prompt := &survey.Select{
		Message: "Choose a project",
		Options: names,
	}

	survey.AskOne(prompt, &selection)

	return selection
}

func resolve(projectName string) (ResolvedProject, error) {
	projects, err := apitesterClient.GetProjects(context.Background())
	if projectName == "" {
		names := []string{}
		for _, p := range projects {
			names = append(names, p.Name)
		}
		projectName = projectSurvey(names)
	}
	if err != nil {
		return ResolvedProject{}, err
	}
	var project apitest.ProjectMeta
	for _, p := range projects {
		if p.Name == projectName {
			project = p
			break
		}
	}
	if project.ID == "" {
		return ResolvedProject{}, fmt.Errorf("Could not find project named %s", projectName)
	}

	hooks, err := apitesterClient.GetHooks(context.Background(), project.ID)
	if err != nil {
		return ResolvedProject{}, err
	}
	if len(hooks) == 0 {
		return ResolvedProject{}, fmt.Errorf("Project named %s has no hooks configured", projectName)
	}

	return ResolvedProject{
		ProjectMeta: project,
		Hooks:   hooks,
	}, nil
}
