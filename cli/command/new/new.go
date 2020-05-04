package new

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/spf13/cobra"
	"github.com/tj/survey"
)

var (
	runUse     = "new"
	runShort   = "Start a new project"
	runLong    = `Some long description`
	runExample = "saucectl new"

	argsYes = false

	projectName = ""
	qs          = []*survey.Question{
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
		Framework string `survey:"color"`
	}{}
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     runUse,
		Short:   runShort,
		Long:    runLong,
		Example: runExample,
		Run: func(cmd *cobra.Command, args []string) {
			cli.Logger.Info().Msg("Start New Command")
			checkErr(Run(cmd, cli, args))
			os.Exit(0)
		},
	}

	cmd.Flags().BoolVarP(&argsYes, "yes", "y", false, "if set it runs with default values")
	return cmd
}

func checkErr(e error) {
	if e != nil {
		panic(e)
	}
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
	os.MkdirAll(filepath.Join(cwd, ".sauce"), 0777)
	fc, err := os.Create(filepath.Join(cwd, ".sauce", "config.yml"))
	defer fc.Close()
	if err != nil {
		return err
	}

	configTpl, err := template.New("configTpl").Parse(configTpl)
	if err != nil {
		return err
	}

	wc := bufio.NewWriter(fc)
	if err := configTpl.Execute(wc, answers); err != nil {
		return err
	}
	wc.Flush()

	os.MkdirAll(filepath.Join(cwd, "tests"), 0777)
	ft, err := os.Create(filepath.Join(cwd, "tests", testTpl[answers.Framework].Filename))
	defer ft.Close()
	if err != nil {
		return err
	}

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
