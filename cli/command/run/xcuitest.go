package run

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/cli/command"
	"github.com/saucelabs/saucectl/cli/flags"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/spf13/cobra"
	"os"
)

// xcFlags contains all XCUITest related flags that are set when 'run' is invoked.
var xcFlags = xcuitestFlags{}

type xcuitestFlags struct {
	Name        string
	App         string
	TestApp     string
	TestOptions xcuitest.TestOptions
	Device      flags.Device
}

// NewXCUITestCmd creates the 'run' command for XCUITest.
func NewXCUITestCmd(cli *command.SauceCtlCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:              "xcuitest",
		Short:            "Run xcuitest tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := runXCUITestCmd(cmd)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
				sentry.CaptureError(err, sentry.Scope{
					Username:   credentials.Get().Username,
					ConfigFile: gFlags.cfgFilePath,
				})
			}
			os.Exit(exitCode)
		},
	}

	f := cmd.Flags()
	f.StringVar(&xcFlags.Name, "name", "", "Sets the name of job as it will appear on Sauce Labs")
	f.StringVar(&xcFlags.App, "app", "", "Specifies the app under test")
	f.StringVar(&xcFlags.TestApp, "testApp", "", "Specifies the test app")

	// Test Options
	f.StringSliceVar(&xcFlags.TestOptions.Class, "testOptions.class", []string{}, "Include classes")

	// Devices (no simulators)
	f.Var(&xcFlags.Device, "device", "Specifies the device to use for testing")

	return cmd
}

// runEspressoCmd runs the xcuitest 'run' command.
func runXCUITestCmd(cmd *cobra.Command) (int, error) {
	if typeDef.Kind == config.KindXcuitest && typeDef.APIVersion == config.VersionV1Alpha {
		return runXcuitest(cmd, tcClient, restoClient, rdcClient, appsClient)
	}

	return 1, errors.New("unknown framework configuration")
}

func runXcuitest(cmd *cobra.Command, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as appstore.AppStore) (int, error) {
	p, err := xcuitest.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyDefaultValues(&p.Sauce)
	overrideCliParameters(cmd, &p.Sauce, &p.Artifacts)
	applyXCUITestFlags(&p)

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", gFlags.regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	xcuitest.SetDeviceDefaultValues(&p)
	err = xcuitest.Validate(p)
	if err != nil {
		return 1, err
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterXcuitestSuite(&p); err != nil {
			return 1, err
		}
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()
	rc.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download
	rc.ArtifactConfig = p.Artifacts.Download

	return runXcuitestInCloud(p, regio, tc, rs, rc, as)
}

func runXcuitestInCloud(p xcuitest.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Running XCUITest in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.XcuitestRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:       &as,
			JobStarter:            &tc,
			JobReader:             &rs,
			RDCJobReader:          &rc,
			JobStopper:            &rs,
			JobWriter:             &tc,
			CCYReader:             &rs,
			TunnelService:         &rs,
			Region:                regio,
			ShowConsoleLog:        false,
			ArtifactDownloader:    &rs,
			RDCArtifactDownloader: &rc,
			DryRun:                gFlags.dryRun,
		},
	}
	return r.RunProject()
}

func filterXcuitestSuite(c *xcuitest.Project) error {
	for _, s := range c.Suites {
		if s.Name == gFlags.suiteName {
			c.Suites = []xcuitest.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", gFlags.suiteName)
}

func applyXCUITestFlags(p *xcuitest.Project) {
	if xcFlags.App != "" {
		p.Xcuitest.App = xcFlags.App
	}
	if xcFlags.TestApp != "" {
		p.Xcuitest.TestApp = xcFlags.TestApp
	}

	// No name, no adhoc suite.
	if xcFlags.Name != "" {
		setXCUITestAdhocSuite(p)
	}
}

func setXCUITestAdhocSuite(p *xcuitest.Project) {
	var dd []config.Device
	if xcFlags.Device.Changed {
		dd = append(dd, xcFlags.Device.Device)
	}

	s := xcuitest.Suite{
		Name:        xcFlags.Name,
		Devices:     dd,
		TestOptions: xcFlags.TestOptions,
	}
	p.Suites = []xcuitest.Suite{s}
}
