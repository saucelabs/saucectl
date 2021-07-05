package ini

import (
	"errors"
	"fmt"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/puppeteer"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

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
)

type initConfig struct {
	batchMode bool

	frameworkName    string
	frameworkVersion string
	cypressJSON      string
	app              string
	testApp          string
	platformName     string
	mode             string
	browserName      string
	region           string
	artifactWhen     config.When
	artifactWhenStr  string
	device           config.Device
	emulator         config.Emulator
	deviceFlag       flags.Device
	emulatorFlag     flags.Emulator
	concurrency      int
	username         string
	accessKey        string
}

var (
	testComposerTimeout = 5 * time.Second
	rdcTimeout          = 5 * time.Second
	restoTimeout        = 5 * time.Second
)

// Command creates the `init` command
func Command() *cobra.Command {
	initCfg := &initConfig{}

	cmd := &cobra.Command{
		Use:     initUse,
		Short:   initShort,
		Long:    initLong,
		Example: initExample,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Start Init Command")
			err := Run(cmd, initCfg)
			if err != nil {
				log.Err(err).Msg("failed to execute init command")
				sentry.CaptureError(err, sentry.Scope{})
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&initCfg.username, "username", "u", "", "username to use")
	cmd.Flags().StringVarP(&initCfg.accessKey, "accessKey", "a", "", "access key for the Sauce Labs account making the request")
	cmd.Flags().StringVarP(&initCfg.region, "region", "r", "us-west-1", "region to use")
	cmd.Flags().StringVarP(&initCfg.frameworkName, "framework", "f", "", "framework to configure")
	cmd.Flags().StringVarP(&initCfg.frameworkVersion, "frameworkVersion", "v", "", "framework version to be used")
	cmd.Flags().StringVar(&initCfg.cypressJSON, "cypress.config", "", "path to cypress.json file (cypress only)")
	cmd.Flags().StringVar(&initCfg.app, "app", "", "path to application to test (espresso/xcuitest only)")
	cmd.Flags().StringVarP(&initCfg.testApp, "testApp", "t", "", "path to test application (espresso/xcuitest only)")
	cmd.Flags().StringVarP(&initCfg.platformName, "platformName", "p", "", "Specified platform name")
	cmd.Flags().StringVarP(&initCfg.browserName, "browserName", "b", "", "Specifies browser name")
	cmd.Flags().StringVar(&initCfg.artifactWhenStr, "artifacts.download.when", "fail", "defines when to download artifacts")
	cmd.Flags().Var(&initCfg.emulatorFlag, "emulator", "Specifies the emulator to use for testing")
	cmd.Flags().Var(&initCfg.deviceFlag, "device", "Specifies the device to use for testing")
	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, initCfg *initConfig) error {
	if cmd.Flags().Changed("framework") {
		return batchMode(cmd, initCfg)
	}
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
	initCfg, err = ini.configure()
	if err != nil {
		return err
	}
	initCfg.region = regio
	initCfg.concurrency = concurrency.Min(ini.ccyReader, 10)
	if initCfg.concurrency == 0 {
		initCfg.concurrency = 1
	}

	files, err := saveConfigurationFiles(initCfg)
	if err != nil {
		return err
	}
	displaySummary(files)
	return nil
}

func batchMode(cmd *cobra.Command, initCfg *initConfig) error {
	stdio := terminal.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
	creds := credentials.Get()
	if !creds.IsValid() {
		return errors.New("no credentials available")
	}

	ini := newInitializer(stdio, creds, initCfg.region)
	initCfg.batchMode = true

	var errs []error
	switch initCfg.frameworkName {
	case cypress.Kind:
		initCfg, errs = ini.initializeBatchCypress(initCfg)
	case espresso.Kind:
		initCfg, errs = ini.initializeBatchEspresso(cmd.Flags(), initCfg)
	case playwright.Kind:
		initCfg, errs = ini.initializeBatchPlaywright(initCfg)
	case puppeteer.Kind:
		initCfg, errs = ini.initializeBatchPuppeteer(initCfg)
	case testcafe.Kind:
		initCfg, errs = ini.initializeBatchTestcafe(initCfg)
	case xcuitest.Kind:
		initCfg, errs = ini.initializeBatchXcuitest(cmd.Flags(), initCfg)
	default:
		println()
		color.HiRed("Invalid framework selected")
		println()
		return errors.New("invalid framework selected")
	}
	if len(errs) > 0 {
		fmt.Printf("%d errors occured:\n", len(errs))
		for _, err := range errs {
			fmt.Printf("- %v\n", err)
		}
		return fmt.Errorf("%s: %d errors occured", initCfg.frameworkName, len(errs))
	}
	initCfg.concurrency = concurrency.Min(ini.ccyReader, 10)
	if initCfg.concurrency == 0 {
		initCfg.concurrency = 1
	}

	files, err := saveConfigurationFiles(initCfg)
	if err != nil {
		return err
	}
	displaySummary(files)
	return nil
}
