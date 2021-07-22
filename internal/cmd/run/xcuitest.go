package run

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type xcuitestFlags struct {
	Device flags.Device
}

// NewXCUITestCmd creates the 'run' command for XCUITest.
func NewXCUITestCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
	lflags := xcuitestFlags{}

	cmd := &cobra.Command{
		Use:              "xcuitest",
		Short:            "Run xcuitest tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
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

	sc.Fset = cmd.Flags()
	sc.String("name", "suite.name", "", "Sets the name of the job as it will appear on Sauce Labs")
	sc.String("app", "xcuitest.app", "", "Specifies the app under test")
	sc.String("testApp", "xcuitest.testApp", "", "Specifies the test app")
	sc.StringSlice("otherApps", "xcuitest.otherApps", []string{}, "Specifies any additional apps that are installed alongside the main app")

	// Test Options
	sc.StringSlice("testOptions.class", "suite.testOptions.class", []string{}, "Include classes")

	// Devices (no simulators)
	cmd.Flags().Var(&lflags.Device, "device", "Specifies the device to use for testing")

	return cmd
}

func runXcuitest(cmd *cobra.Command, flags xcuitestFlags, tc testcomposer.Client, rs resto.Client, rc rdc.Client,
	as appstore.AppStore) (int, error) {
	p, err := xcuitest.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CommandLine = buildCommandLineFlagsMap(cmd)
	p.Sauce.Metadata.ExpandEnv()

	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	if err := applyXCUITestFlags(&p, flags); err != nil {
		return 1, err
	}
	xcuitest.SetDefaults(&p)

	if err := xcuitest.Validate(p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)

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
		},
	}
	return r.RunProject()
}

func applyXCUITestFlags(p *xcuitest.Project, flags xcuitestFlags) error {
	if gFlags.selectedSuite != "" {
		if err := xcuitest.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	if p.Suite.Name == "" {
		return nil
	}

	if flags.Device.Changed {
		p.Suite.Devices = append(p.Suite.Devices, flags.Device.Device)
	}

	p.Suites = []xcuitest.Suite{p.Suite}

	return nil
}
