package run

import (
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/sentry"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/usage"
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
		Long:             "Unlike 'saucectl run', this command allows you to bypass the config file partially or entirely by configuring an adhoc suite (--name) via flags.",
		Example:          `saucectl run espresso -c "" --name "My Suite" --app app.apk --testApp testApp.apk --otherApps=a.apk,b.apk --device name="Google Pixel.*",platformVersion=14.0,carrierConnectivity=false,deviceType=PHONE,private=false --emulator name="Android Emulator",platformVersion=8.0`,
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
	sc.String("name", "suite::name", "", "Sets the name of the job as it will appear on Sauce Labs")
	sc.String("app", "espresso::app", "", "Specifies the app under test")
	sc.String("testApp", "espresso::testApp", "", "Specifies the test app")
	sc.StringSlice("otherApps", "espresso::otherApps", []string{}, "Specifies any additional apps that are installed alongside the main app")

	// Test Options
	sc.StringSlice("testOptions.class", "suite::testOptions::class", []string{}, "Only run the specified classes. Requires --name to be set.")
	sc.StringSlice("testOptions.notClass", "suite::testOptions::notClass", []string{}, "Run all classes except those specified here. Requires --name to be set.")
	sc.String("testOptions.package", "suite::testOptions::package", "", "Include package. Requires --name to be set.")
	sc.String("testOptions.size", "suite::testOptions::size", "", "Include tests based on size. Requires --name to be set.")
	sc.String("testOptions.annotation", "suite::testOptions::annotation", "", "Include tests based on the annotation. Requires --name to be set.")
	sc.String("testOptions.notAnnotation", "suite::testOptions::notAnnotation", "", "Run all tests except those with this annotation. Requires --name to be set.")
	sc.Int("testOptions.numShards", "suite::testOptions::numShards", 0, "Total number of shards. Requires --name to be set.")
	sc.Bool("testOptions.useTestOrchestrator", "suite::testOptions::useTestOrchestrator", false, "Set the instrumentation to start with Test Orchestrator. Requires --name to be set.")

	// Emulators and Devices
	cmd.Flags().Var(&lflags.Emulator, "emulator", "Specifies the emulator to use for testing. Requires --name to be set.")
	cmd.Flags().Var(&lflags.Device, "device", "Specifies the device to use for testing. Requires --name to be set.")

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

	tracker := segment.New(!gFlags.disableUsageMetrics)

	defer func() {
		props := usage.Properties{}
		props.SetFramework("espresso").SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).SetArtifacts(p.Artifacts).
			SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults)
		tracker.Collect(strings.Title(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

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
			MetadataService:       &tc,
			Region:                regio,
			ShowConsoleLog:        p.ShowConsoleLog,
			ArtifactDownloader:    &rs,
			RDCArtifactDownloader: &rc,
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &tc,
				"espresso", "sauce"),
			Framework: framework.Framework{Name: espresso.Kind},
			Async:     gFlags.async,
			FailFast:  gFlags.failFast,
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
		isErr := len(p.Suite.TestOptions.Class) != 0 ||
			len(p.Suite.TestOptions.NotClass) != 0 ||
			p.Suite.TestOptions.Package != "" ||
			p.Suite.TestOptions.NotPackage != "" ||
			p.Suite.TestOptions.Size != "" ||
			p.Suite.TestOptions.Annotation != "" ||
			p.Suite.TestOptions.NotAnnotation != "" ||
			p.Suite.TestOptions.NumShards != 0 ||
			p.Suite.TestOptions.UseTestOrchestrator ||
			flags.Device.Changed ||
			flags.Emulator.Changed

		if isErr {
			return ErrEmptySuiteName
		}

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
