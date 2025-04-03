package ini

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/xctest"
	"github.com/saucelabs/saucectl/internal/xcuitest"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	initUse     = "init"
	initShort   = "bootstrap project"
	initLong    = "bootstrap an existing project for Sauce Labs"
	initExample = "saucectl init"
)

var noPrompt = false
var regionName = "us-west-1"

type initConfig struct {
	frameworkName     string
	frameworkVersion  string
	cypressConfigFile string
	app               string
	testApp           string
	xctestRunFile     string
	otherApps         []string
	platformName      string
	browserName       string
	region            string
	artifactWhen      config.When
	artifactWhenStr   string
	device            config.Device
	emulator          config.Emulator
	simulator         config.Simulator
	deviceFlag        flags.Device
	emulatorFlag      flags.Emulator
	simulatorFlag     flags.Simulator
	concurrency       int
	playwrightProject string
	testMatch         []string
}

var (
	testComposerTimeout = 5 * time.Second
	rdcTimeout          = 5 * time.Second
	restoTimeout        = 5 * time.Second
)

func Command(preRun func(cmd *cobra.Command, args []string)) *cobra.Command {
	cmd := &cobra.Command{
		Use:              initUse,
		Short:            initShort,
		Long:             initLong,
		Example:          initExample,
		SilenceUsage:     true,
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				preRun(cmd, args)
			}

			err := http.CheckProxy()
			if err != nil {
				return fmt.Errorf("invalid HTTP_PROXY value")
			}
			return nil
		},
	}

	cmd.AddCommand(
		CypressCmd(),
		EspressoCmd(),
		PlaywrightCmd(),
		TestCafeCmd(),
		XCUITestCmd(),
		XCTestCmd(),
	)

	flags := cmd.PersistentFlags()

	flags.BoolVar(&noPrompt, "no-prompt", false, "Disable interactive prompts.")
	flags.StringVarP(&regionName, "region", "r", "", "Sauce Labs region. Options: us-west-1, eu-central-1.")

	return cmd
}

// Run runs the command
func Run(cmd *cobra.Command, cfg *initConfig) error {
	cfg.region = regionName

	if noPrompt {
		return noPromptMode(cmd, cfg)
	}
	stdio := terminal.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}

	creds := credentials.Get()
	if !creds.IsSet() {
		var err error
		creds, err = askCredentials(stdio)
		if err != nil {
			return err
		}
		if err = credentials.ToFile(creds); err != nil {
			return err
		}
	}

	var err error
	if cfg.region == "" {
		cfg.region, err = askRegion(stdio)
		if err != nil {
			return err
		}
	}

	ini := newInitializer(stdio, creds, cfg)

	err = ini.checkCredentials(cmd.Context(), regionName)
	if err != nil {
		return err
	}

	err = ini.configure(cmd.Context())
	if err != nil {
		return err
	}

	ccy, err := ini.userService.Concurrency(cmd.Context())
	if err != nil {
		println()
		color.HiRed("Unable to determine your exact allowed concurrency.\n")
		color.HiBlue("Using 1 as default value.\n")
		println()
		ccy.Org.Allowed.VDC = 1
	}
	cfg.concurrency = ccy.Org.Allowed.VDC

	files, err := saveConfigurationFiles(cfg)
	if err != nil {
		return err
	}
	displaySummary(files)
	return nil
}

func noPromptMode(cmd *cobra.Command, cfg *initConfig) error {
	stdio := terminal.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
	creds := credentials.Get()
	if !creds.IsSet() {
		return errors.New(msg.EmptyCredentials)
	}
	if cfg.region == "" {
		return errors.New(msg.MissingRegion)
	}

	ini := newInitializer(stdio, creds, cfg)

	var errs []error
	switch cfg.frameworkName {
	case cypress.Kind:
		errs = ini.initializeBatchCypress(cmd.Context())
	case espresso.Kind:
		errs = ini.initializeBatchEspresso(cmd.Context(), cmd.Flags())
	case playwright.Kind:
		errs = ini.initializeBatchPlaywright(cmd.Context())
	case testcafe.Kind:
		errs = ini.initializeBatchTestcafe(cmd.Context())
	case xcuitest.Kind:
		errs = ini.initializeBatchXcuitest(cmd.Flags())
	case xctest.Kind:
		errs = ini.initializeBatchXctest(cmd.Flags())
	default:
		println()
		color.HiRed("Invalid framework selected")
		println()
		return errors.New(msg.InvalidSelectedFramework)
	}
	if len(errs) > 0 {
		fmt.Printf("%d errors occured:\n", len(errs))
		for _, err := range errs {
			fmt.Printf("- %v\n", err)
		}
		return fmt.Errorf("%s: %d errors occured", cfg.frameworkName, len(errs))
	}

	ccy, err := ini.userService.Concurrency(cmd.Context())
	if err != nil {
		println()
		color.HiRed("Unable to determine your exact allowed concurrency.\n")
		color.HiBlue("Using 1 as default value.\n")
		println()
		ccy.Org.Allowed.VDC = 1
	}
	cfg.concurrency = ccy.Org.Allowed.VDC

	files, err := saveConfigurationFiles(cfg)
	if err != nil {
		return err
	}
	displaySummary(files)
	return nil
}
