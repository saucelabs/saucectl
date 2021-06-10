package init

import (
	"errors"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/mocks"
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

type FrameworkInfoReader interface {
	Frameworks() ([]string, error)
	Versions(frameworkName, region string) ([]string, error)
	Platforms(frameworkName, region, frameworkVersion string) ([]string, error)
	Browsers(frameworkName, region, frameworkVersion, platformName string) ([]string, error)
}

type initiator struct {
	stdio      terminal.Stdio
	infoReader FrameworkInfoReader
}

type initConfig struct {
	frameworkName    string
	frameworkVersion string
	cypressJson      string
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
}

var configurators = map[string]func(cfg *initConfig) interface{}{
	"cypress":    configureCypress,
	"espresso":   configureEspresso,
	"playwright": configurePlaywright,
	"puppeteer":  configurePuppeteer,
	"testcafe":   configureTestcafe,
	"xcuitest":   configureXCUITest,
}

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	// FIXME: Provision using API
	ini := initiator{
		stdio:      terminal.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr},
		infoReader: &mocks.FakeFrameworkInfoReader{},
	}

	initCfg, err := ini.configure()
	if err != nil {
		return err
	}

	if f, ok := configurators[initCfg.frameworkName]; ok {
		return saveConfiguration(f(initCfg))
	}
	log.Error().Msgf("%s: not implemented", strings.ToLower(initCfg.frameworkName))
	return errors.New("unsupported framework")
}
