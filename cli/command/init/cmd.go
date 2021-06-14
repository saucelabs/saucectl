package init

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/vmd"
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

type initiator struct {
	stdio        terminal.Stdio
	infoReader   framework.MetadataService
	deviceReader devices.Reader
	vmdReader    vmd.Reader

	frameworks        []string
	frameworkMetadata []framework.Metadata
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

var (
	testComposerTimeout = 5 * time.Second
	rdcTimeout          = 5 * time.Second
)

// Run runs the command
func Run(cmd *cobra.Command, cli *command.SauceCtlCli, args []string) error {
	creds := credentials.Get()
	tc := testcomposer.Client{
		HTTPClient:  &http.Client{Timeout: testComposerTimeout},
		URL:         region.FromString("us-west-1").APIBaseURL(), // Will updated as soon
		Credentials: creds,
	}

	rc := rdc.Client{
		HTTPClient: &http.Client{Timeout: rdcTimeout},
		URL:        region.FromString("us-west-1").APIBaseURL(), // Will updated as soon
		Username:   creds.Username,
		AccessKey:  creds.AccessKey,
	}

	ini := initiator{
		stdio:        terminal.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr},
		infoReader:   &tc,
		deviceReader: &rc,
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
