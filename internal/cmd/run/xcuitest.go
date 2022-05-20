package run

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"

	"github.com/saucelabs/saucectl/internal/appstore"
	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/download"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/rdc"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/resto"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/testcomposer"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/xcuitest"
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
		Short:            "Run xcuitest tests.",
		Long:             "Unlike 'saucectl run', this command allows you to bypass the config file partially or entirely by configuring an adhoc suite (--name) via flags.",
		Example:          `saucectl run xcuitest -c "" --name "My Suite" --app app.ipa --testApp testApp.ipa --otherApps=a.ipa,b.ipa --device name="iPhone.*",platformVersion=14.0,carrierConnectivity=false,deviceType=PHONE,private=false`,
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
				backtrace.Report(err, map[string]interface{}{
					"username": credentials.Get().Username,
				}, gFlags.cfgFilePath)
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()
	sc.String("name", "suite::name", "", "Creates a new adhoc suite with this name. Suites defined in the config will be ignored.")
	sc.String("app", "xcuitest::app", "", "Specifies the app under test")
	sc.String("testApp", "xcuitest::testApp", "", "Specifies the test app")
	sc.StringSlice("otherApps", "xcuitest::otherApps", []string{}, "Specifies any additional apps that are installed alongside the main app")

	// Test Options
	sc.StringSlice("testOptions.class", "suite::testOptions::class", []string{}, "Only run the specified classes. Requires --name to be set.")
	sc.StringSlice("testOptions.notClass", "suite::testOptions::notClass", []string{}, "Run all classes except those specified here. Requires --name to be set.")

	// Devices (no simulators)
	cmd.Flags().Var(&lflags.Device, "device", "Specifies the device to use for testing. Requires --name to be set.")

	// Overwrite devices settings
	sc.Bool("audioCapture", "suite::appSettings::audioCapture", false, "Overwrite app settings for real device to capture audio.")
	sc.Bool("networkCapture", "suite::appSettings::instrumentation::networkCapture", false, "Overwrite app settings for real device to capture network.")

	return cmd
}

func runXcuitest(cmd *cobra.Command, xcuiFlags xcuitestFlags, tc testcomposer.Client, rs resto.Client, rc rdc.Client,
	as appstore.AppStore) (int, error) {
	p, err := xcuitest.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())

	if err := applyXCUITestFlags(&p, xcuiFlags); err != nil {
		return 1, err
	}
	xcuitest.SetDefaults(&p)

	if err := xcuitest.Validate(p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)

	webdriverClient.URL = regio.WebDriverBaseURL()
	tc.URL = regio.APIBaseURL()
	rs.URL = regio.APIBaseURL()
	as.URL = regio.APIBaseURL()
	rc.URL = regio.APIBaseURL()

	rs.ArtifactConfig = p.Artifacts.Download
	rc.ArtifactConfig = p.Artifacts.Download

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := segment.New(!gFlags.disableUsageMetrics)

	defer func() {
		props := usage.Properties{}
		props.SetFramework("xcuitest").SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).SetArtifacts(p.Artifacts).
			SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).SetSlack(p.Notifications.Slack)
		tracker.Collect(cases.Title(language.English).String(fullCommandName(cmd)), props)
		_ = tracker.Close()
	}()

	if p.Artifacts.Cleanup {
		download.Cleanup(p.Artifacts.Download.Directory)
	}

	return runXcuitestInCloud(p, regio, tc, rs, rc, as)
}

func runXcuitestInCloud(p xcuitest.Project, regio region.Region, tc testcomposer.Client, rs resto.Client, rc rdc.Client, as appstore.AppStore) (int, error) {
	log.Info().Msg("Running XCUITest in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.XcuitestRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader:       &as,
			JobStarter:            &webdriverClient,
			RDCJobStarter:         &rc,
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
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &tc, &rs,
				"xcuitest", "sauce"),
			Framework: framework.Framework{Name: xcuitest.Kind},
			Async:     gFlags.async,
			FailFast:  gFlags.failFast,
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
		isErr := len(p.Suite.TestOptions.Class) != 0 ||
			len(p.Suite.TestOptions.NotClass) != 0 ||
			flags.Device.Changed

		if isErr {
			return ErrEmptySuiteName
		}

		return nil
	}

	if flags.Device.Changed {
		p.Suite.Devices = append(p.Suite.Devices, flags.Device.Device)
	}

	p.Suites = []xcuitest.Suite{p.Suite}

	return nil
}
