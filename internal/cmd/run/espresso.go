package run

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/spf13/cobra"
)

type espressoFlags struct {
	Name        string
	App         string
	TestApp     string
	TestOptions espresso.TestOptions
	Emulator    flags.Emulator
	Device      flags.Device
}

// NewEspressoCmd creates the 'run' command for espresso.
func NewEspressoCmd() *cobra.Command {
	lflags := espressoFlags{}

	cmd := &cobra.Command{
		Use:              "espresso",
		Short:            "Run espresso tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := runEspresso(cmd, lflags, tcClient, restoClient, rdcClient, appsClient)
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
	f.StringSliceVar(&lflags.TestOptions.NotClass, "testOptions.notClass", []string{}, "Exclude classes")
	f.StringVar(&lflags.TestOptions.Package, "testOptions.package", "", "Include package")
	f.StringVar(&lflags.TestOptions.Size, "testOptions.size", "", "Include tests based on size")
	f.StringVar(&lflags.TestOptions.Annotation, "testOptions.annotation", "", "Include tests based on the annotation")
	var shardIndex, numShards int
	f.IntVar(&numShards, "testOptions.numShards", 0, "Total number of shards")
	f.IntVar(&shardIndex, "testOptions.shardIndex", -1, "The shard index for this particular run")

	lflags.TestOptions.NumShards = &numShards
	if shardIndex >= 0 {
		lflags.TestOptions.ShardIndex = &shardIndex
	}

	// Emulators and Devices
	f.Var(&lflags.Emulator, "emulator", "Specifies the emulator to use for testing")
	f.Var(&lflags.Device, "device", "Specifies the device to use for testing")

	return cmd
}

func runEspresso(cmd *cobra.Command, flags espressoFlags, tc testcomposer.Client, rs resto.Client, rc rdc.Client,
	as appstore.AppStore) (int, error) {
	p, err := espresso.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.Sauce.Metadata.ExpandEnv()
	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	applyEspressoFlags(&p, flags)

	regio := region.FromString(p.Sauce.Region)
	if regio == region.None {
		log.Error().Str("region", gFlags.regionFlag).Msg("Unable to determine sauce region.")
		return 1, errors.New("no sauce region set")
	}

	err = espresso.Validate(p)
	if err != nil {
		return 1, err
	}

	if cmd.Flags().Lookup("suite").Changed {
		if err := filterEspressoSuite(&p); err != nil {
			return 1, err
		}
	}

	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()
	rc.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download
	rc.ArtifactConfig = p.Artifacts.Download

	return runEspressoInCloud(p, regio, tc, rs, rc, as)
}

func runEspressoInCloud(p espresso.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Running Espresso in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.EspressoRunner{
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

func filterEspressoSuite(c *espresso.Project) error {
	for _, s := range c.Suites {
		if s.Name == gFlags.suiteName {
			c.Suites = []espresso.Suite{s}
			return nil
		}
	}
	return fmt.Errorf("suite name '%s' is invalid", gFlags.suiteName)
}

func applyEspressoFlags(p *espresso.Project, flags espressoFlags) {
	if flags.App != "" {
		p.Espresso.App = flags.App
	}
	if flags.TestApp != "" {
		p.Espresso.TestApp = flags.TestApp
	}

	// No name, no adhoc suite.
	if flags.Name != "" {
		setAdhocSuite(p, flags)
	}
}

func setAdhocSuite(p *espresso.Project, flags espressoFlags) {
	var dd []config.Device
	if flags.Device.Changed {
		dd = append(dd, flags.Device.Device)
	}

	var ee []config.Emulator
	if flags.Emulator.Changed {
		ee = append(ee, flags.Emulator.Emulator)
	}

	s := espresso.Suite{
		Name:        flags.Name,
		Devices:     dd,
		Emulators:   ee,
		TestOptions: flags.TestOptions,
	}
	p.Suites = []espresso.Suite{s}
}
