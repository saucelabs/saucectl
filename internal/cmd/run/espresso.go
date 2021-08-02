package run

import (
	"os"

	"github.com/spf13/pflag"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/appstore"
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
	Emulator flags.Emulator
	Device   flags.Device
}

// NewEspressoCmd creates the 'run' command for espresso.
func NewEspressoCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
	lflags := espressoFlags{}

	cmd := &cobra.Command{
		Use:              "espresso",
		Short:            "Run espresso tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
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

	sc.Fset = cmd.Flags()
	sc.String("name", "suite.name", "", "Sets the name of the job as it will appear on Sauce Labs")
	sc.String("app", "espresso.app", "", "Specifies the app under test")
	sc.String("testApp", "espresso.testApp", "", "Specifies the test app")
	sc.StringSlice("otherApps", "espresso.otherApps", []string{}, "Specifies any additional apps that are installed alongside the main app")

	// Test Options
	sc.StringSlice("testOptions.class", "suite.testOptions.class", []string{}, "Only run the specified classes")
	sc.StringSlice("testOptions.notClass", "suite.testOptions.notClass", []string{}, "Run all classes except those specified here")
	sc.String("testOptions.package", "suite.testOptions.package", "", "Include package")
	sc.String("testOptions.size", "suite.testOptions.size", "", "Include tests based on size")
	sc.String("testOptions.annotation", "suite.testOptions.annotation", "", "Include tests based on the annotation")
	sc.Int("testOptions.numShards", "suite.testOptions.numShards", 0, "Total number of shards")
	sc.Bool("testOptions.useTestOrchestrator", "suite.testOptions.useTestOrchestrator", false, "Set the instrumentation to start with Test Orchestrator")

	// Emulators and Devices
	cmd.Flags().Var(&lflags.Emulator, "emulator", "Specifies the emulator to use for testing")
	cmd.Flags().Var(&lflags.Device, "device", "Specifies the device to use for testing")

	return cmd
}

func runEspresso(cmd *cobra.Command, espressoFlags espressoFlags, tc testcomposer.Client, rs resto.Client, rc rdc.Client,
	as appstore.AppStore) (int, error) {
	p, err := espresso.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())
	p.Sauce.Metadata.ExpandEnv()

	applyGlobalFlags(cmd, &p.Sauce, &p.Artifacts)
	if err := applyEspressoFlags(&p, espressoFlags); err != nil {
		return 1, err
	}
	espresso.SetDefaults(&p)

	if err := espresso.Validate(p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)

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
		},
	}

	return r.RunProject()
}

func applyEspressoFlags(p *espresso.Project, flags espressoFlags) error {
	if gFlags.selectedSuite != "" {
		if err := espresso.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	if p.Suite.Name == "" {
		return nil
	}

	if flags.Device.Changed {
		p.Suite.Devices = append(p.Suite.Devices, flags.Device.Device)
	}

	if flags.Emulator.Changed {
		p.Suite.Emulators = append(p.Suite.Emulators, flags.Emulator.Emulator)
	}

	p.Suites = []espresso.Suite{p.Suite}

	return nil
}
