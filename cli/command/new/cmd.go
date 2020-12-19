package new

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/config"
	"github.com/saucelabs/saucectl/cli/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/yaml"
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

	frameworks = []struct {
		Framework  string
		GithubOrg  string
		GithubRepo string
	}{
		{"Puppeteer", "saucelabs", "sauce-puppeteer-runner"},
		{"Playwright", "saucelabs", "sauce-playwright-runner"},
		{"Testcafe", "saucelabs", "sauce-testcafe-runner"},
		{"Cypress", "saucelabs", "sauce-cypress-runner"},
	}

	qs = []*survey.Question{
		{
			Name: "framework",
			Prompt: &survey.Select{
				Message: "Choose a framework:",
				Options: frameworkChoices(),
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
	cfgFilePath := ".sauce/config.yml"

	org, repo, err := getRepositoryValues(answers.Framework)
	if err != nil {
		return err
	}

	err = FetchAndExtractTemplate(org, repo)
	if err != nil {
		return fmt.Errorf("no template available for %s (%s)", answers.Framework, err)
	}

	err = updateRegion(cfgFilePath, answers.Region)
	if err != nil {
		return err
	}

	fmt.Println("\nNew project bootstrapped successfully! You can now run:\n$ saucectl run")

	creds := credentials.Get()
	if creds == nil {
		fmt.Println("\nIt looks you have not configured your SauceLab account !\nTo enjoy SauceLabs capabilities, configure your account by running:\n$ saucectl configure")
	}
	return nil
}

// Overwrite the region from the template archive
func updateRegion(cfgFile string, region string) error {
	cwd, _ := os.Getwd()
	cfgPath := filepath.Join(cwd, cfgFile)

	d, err := config.Describe(cfgFile)
	if err != nil {
		return err
	}

	if d.Kind == config.KindCypress && d.APIVersion == config.VersionV1Alpha {
		c, err := cypress.FromFile(cfgFile)
		if err != nil {
			return err
		}
		c.Sauce.Region = region
		return yaml.WriteFile(cfgPath, c)
	}
	if d.Kind == config.KindPlaywright && d.APIVersion == config.VersionV1Alpha {
		c, err := playwright.FromFile(cfgFile)
		if err != nil {
			return err
		}
		c.Sauce.Region = region
		return yaml.WriteFile(cfgPath, c)
	}
	c, err := config.NewJobConfiguration(cfgPath)
	if err != nil {
		return err
	}
	c.Sauce.Region = region
	return yaml.WriteFile(cfgPath, c)
}

// Create choice list
func frameworkChoices() []string {
	var frameworkNames []string
	for _, framework := range frameworks {
		frameworkNames = append(frameworkNames, framework.Framework)
	}
	return frameworkNames
}

func getRepositoryValues(framework string) (string, string, error) {
	for _, repo := range frameworks {
		if strings.ToLower(repo.Framework) == framework {
			return repo.GithubOrg, repo.GithubRepo, nil
		}
	}
	return "", "", errors.New("unknown framework")
}
