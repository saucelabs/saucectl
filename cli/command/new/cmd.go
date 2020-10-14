package new

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/spf13/cobra"
	"github.com/tj/survey"
	"os"
	"path/filepath"
	"strings"
)

var (
	newUse     = "new"
	newShort   = "Start a new project"
	newLong    = `Some long description`
	newExample = "saucectl new"

	argsYes = false

	frameworks = map[string]struct {
		GithubOrg  string
		GithubRepo string
	}{
		"Playwright": {"saucelabs", "sauce-playwright-runner"},
		"Puppeteer":  {"saucelabs", "sauce-puppeteer-runner"},
		"Testcafe":   {"saucelabs", "sauce-testcafe-runner"},
		"Cypress":    {"saucelabs", "sauce-cypress-runner"},
	}

	qs = []*survey.Question{
		{
			Name: "framework",
			Prompt: &survey.Select{
				Message: "Choose a framework:",
				Options: []string{"Puppeteer", "Playwright", "Testcafe", "Cypress"},
				Default: "Puppeteer",
			},
		},
		{
			Name: "region",
			Prompt: &survey.Select{
				Message: "Choose the sauce labs region:",
				Options: []string{"us-west-1", "eu-central-1"},
				Default: "us-west-1",
			},
		},
	}

	answers = struct {
		Framework string
		Region    string
	}{}
)

// Command creates the `new` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     newUse,
		Short:   newShort,
		Long:    newLong,
		Example: newExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start New Command")
			if err := Run(cmd, cli, args); err != nil {
				log.Err(err).Msg("failed to execute new command")
				os.Exit(1)
			}
		},
	}

	cmd.Flags().BoolVarP(&argsYes, "yes", "y", false, "if set it runs with default values")
	return cmd
}

// Run starts the new command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = survey.Ask(qs, &answers)
	if err != nil {
		return err
	}

	answers.Framework = strings.ToLower(answers.Framework)
	if err := os.MkdirAll(filepath.Join(cwd, ".sauce"), 0777); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	org, repo, err := getRepositoryValues(answers.Framework)
	if err != nil {
		return err
	}

	err = FetchAndExtractTemplate(org, repo)
	if err != nil {
		fmt.Printf("No template available for %s\n", answers.Framework)
	}

	fmt.Println("\nNew project bootstrapped successfully! You can now run:\n$ saucectl run")
	return nil
}

func getRepositoryValues(framework string) (string, string, error) {
	for key, repo := range frameworks {
		if strings.ToLower(key) == framework {
			return repo.GithubOrg, repo.GithubRepo, nil
		}
	}
	return "", "", errors.New("unknown framework")
}
