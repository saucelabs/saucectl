package init

import (
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/concurrency"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/sentry"
)

var (
	initUse     = "init"
	initShort   = "bootstrap project"
	initLong    = "bootstrap an existing project for Sauce Labs"
	initExample = "saucectl init"

	frameworkName = ""
)

// Command creates the `run` command
func Command(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     initUse,
		Short:   initShort,
		Long:    initLong,
		Example: initExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Init Command")
			err := Run(cmd, cli, args)
			if err != nil {
				log.Err(err).Msg("failed to execute init command")
				sentry.CaptureError(err, sentry.Scope{})
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&frameworkName, "framework", "f", "", "framework to init")
	return cmd
}

type initConfig struct {
	frameworkName    string
	frameworkVersion string
	cypressJSON      string
	rootDir          string
	app              string
	testApp          string
	platformName     string
	mode             string
	browserName      string
	region           string
	artifactWhen     config.When
	device           config.Device
	emulator         config.Emulator
	concurrency      int
}

var (
	testComposerTimeout = 5 * time.Second
	rdcTimeout          = 5 * time.Second
	restoTimeout        = 5 * time.Second
)

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	stdio := terminal.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}

	creds := credentials.Get()
	if !creds.IsValid() {
		var err error
		creds, err = askCredentials(stdio)
		if err != nil {
			return err
		}
		if err = credentials.ToFile(creds); err != nil {
			return err
		}
	}

	regio, err := askRegion(stdio)
	if err != nil {
		return err
	}

	ini := newInitializer(stdio, creds, regio)
	err = ini.checkCredentials()
	if err != nil {
		return err
	}
	initCfg, err := ini.configure()
	if err != nil {
		return err
	}
	initCfg.region = regio
	initCfg.concurrency = concurrency.Min(ini.ccyReader, 10)

	files, err := saveConfigurationFiles(initCfg)
	if err != nil {
		return err
	}
	displaySummary(files)
	return nil
}

