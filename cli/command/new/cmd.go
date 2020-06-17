package new

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/docker"
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
				Options: []string{"Puppeteer", "Playwright"},
				Default: "Puppeteer",
			},
		},
	}

	answers = struct {
		Framework string
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

	if err := writeJobConfig(answers.Framework, fc); err != nil {
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

func writeJobConfig(framework string, w io.Writer) error {
	configTpl, err := template.New("configTpl").Parse(configTpl)
	if err != nil {
		return err
	}

	switch framework {
	case "playwright":
		return configTpl.Execute(w, docker.DefaultPlaywright)
	case "puppeteer":
		return configTpl.Execute(w, docker.DefaultPuppeteer)
	default:
		return errors.New("unknown framework")
	}
}
