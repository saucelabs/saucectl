package run

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
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

type xcuitestFlags struct {
	Name        string
	App         string
	TestApp     string
	TestOptions xcuitest.TestOptions
	Device      flags.Device
}

// NewXCUITestCmd creates the 'run' command for XCUITest.
func NewXCUITestCmd() *cobra.Command {
	lflags := xcuitestFlags{}

	cmd := &cobra.Command{
		Use:              "xcuitest",
		Short:            "Run xcuitest tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := runXcuitest(cmd, lflags, tcClient, restoClient, rdcClient, appsClient)
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
	f.StringVar(&lflags.Name, "name", "", "Sets the name of the job as it will appear on Sauce Labs")
	f.StringVar(&lflags.App, "app", "", "Specifies the app under test")
	f.StringVar(&lflags.TestApp, "testApp", "", "Specifies the test app")

	// Test Options
	f.StringSliceVar(&lflags.TestOptions.Class, "testOptions.class", []string{}, "Include classes")

	// Devices (no simulators)
	f.Var(&lflags.Device, "device", "Specifies the device to use for testing")

	return cmd
}

func runXcuitest(cmd *cobra.Command, flags xcuitestFlags, tc testcomposer.Client, rs resto.Client, rc rdc.Client,
	as appstore.AppStore) (int, error) {
	p, err := xcuitest.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	applyXCUITestFlags(&p, flags)

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

func applyXCUITestFlags(p *xcuitest.Project, flags xcuitestFlags) {
	if flags.App != "" {
		p.Xcuitest.App = flags.App
	}
	if flags.TestApp != "" {
		p.Xcuitest.TestApp = flags.TestApp
	}

	// No name, no adhoc suite.
	if flags.Name != "" {
		setXCUITestAdhocSuite(p, flags)
	}
}

func setXCUITestAdhocSuite(p *xcuitest.Project, flags xcuitestFlags) {
	var dd []config.Device
	if flags.Device.Changed {
		dd = append(dd, flags.Device.Device)
	}

	s := xcuitest.Suite{
		Name:        flags.Name,
		Devices:     dd,
		TestOptions: flags.TestOptions,
	}
	p.Suites = []xcuitest.Suite{s}
}
