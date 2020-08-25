package new

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/docker"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/spf13/cobra"
	"github.com/tj/survey"
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

	fc, err := os.Create(filepath.Join(cwd, ".sauce", "config.yml"))
	if err != nil {
		return err
	}
	defer fc.Close()

	if err := writeJobConfig(answers.Framework, answers.Region, fc); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(cwd, "tests"), 0777); err != nil {
		return err
	}

	ft, err := os.Create(filepath.Join(cwd, "tests", testTpl[answers.Framework].Filename))
	if err != nil {
		return err
	}
	defer ft.Close()

	testTpl, err := template.New("configTpl").Parse(testTpl[answers.Framework].Code)
	if err != nil {
		return err
	}

	wt := bufio.NewWriter(ft)
	if err := testTpl.Execute(wt, answers); err != nil {
		return err
	}
	wt.Flush()

	fmt.Println("\nNew project bootstrapped successfully! You can now run:\n$ saucectl run")
	return nil
}

func writeJobConfig(framework string, region string, w io.Writer) error {
	configTpl, err := template.New("configTpl").Parse(configTpl)
	if err != nil {
		return err
	}

	// TODO(AlexP) Replace template rendering and instead use the JobConfiguration struct directly to render the yaml

	v := struct {
		Name    string
		Version string
		Region  string
	}{
		Region: region,
	}

	var image docker.Image
	switch framework {
	case "playwright":
		image = docker.DefaultPlaywright
	case "puppeteer":
		image = docker.DefaultPuppeteer
	case "testcafe":
		image = docker.DefaultTestcafe
	case "cypress":
		image = docker.DefaultCypress
	default:
		return errors.New("unknown framework")
	}

	v.Name = image.Name
	v.Version = image.Version

	return configTpl.Execute(w, v)
}
