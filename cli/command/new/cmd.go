package new

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/yaml"
	"github.com/spf13/cobra"
	"github.com/tj/survey"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	newUse     = "new"
	newShort   = "Start a new project"
	newLong    = `Some long description`
	newExample = "saucectl new"

	argsYes = false
	
	qs = []*survey.Question{
		{
			Name: "framework",
			Prompt: &survey.Select{
				Message: "Choose a framework:",
				Options: []string{"Cypress", "Playwright", "Puppeteer", "Testcafe"},
				Default: "Cypress",
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
	creds := credentials.Get()
	if creds == nil {
		fmt.Println("\nIt looks you have not configured your SauceLab account !\nTo enjoy SauceLabs capabilities, configure your account by running:\n$ saucectl configure")
		return nil
	}

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

	r := region.FromString(answers.Region)

	tc := testcomposer.Client{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		URL:         r.APIBaseURL(),
		Credentials: *credentials.Get(),
	}

	m, err := tc.Search(cmd.Context(), framework.SearchOptions{
		Name:             answers.Framework,
		FrameworkVersion: "latest",
	})
	if err != nil {
		return err
	}

	org, repo, tag, err := framework.GitReleaseSegments(&m)
	if err != nil {
		return err
	}
	rinfo := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", org, repo, tag)
	log.Info().Str("release", rinfo).Msg("Downloading template.")

	err = FetchAndExtractTemplate(org, repo, tag)
	if err != nil {
		return fmt.Errorf("no template available for %s (%s)", answers.Framework, err)
	}

	err = updateRegion(cfgFilePath, answers.Region)
	if err != nil {
		return err
	}

	fmt.Println("\nNew project bootstrapped successfully! You can now run:\n$ saucectl run")
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
