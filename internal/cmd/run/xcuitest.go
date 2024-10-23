package run

import (
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/http"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/downloader"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

type xcuitestFlags struct {
	Device    flags.Device
	Simulator flags.Simulator
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
		SilenceUsage:     true,
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			exitCode, err := runXcuitest(cmd, lflags, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()
	sc.String("name", "suite::name", "", "Creates a new adhoc suite with this name. Suites defined in the config will be ignored.")
	sc.String("app", "xcuitest::app", "", "Specifies the app under test")
	sc.String("appDescription", "xcuitest::appDescription", "", "Specifies description for the app")
	sc.String("testApp", "xcuitest::testApp", "", "Specifies the test app")
	sc.String("testAppDescription", "xcuitest::testAppDescription", "", "Specifies description for the testApp")
	sc.StringSlice("otherApps", "xcuitest::otherApps", []string{}, "Specifies any additional apps that are installed alongside the main app")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

	sc.String("shard", "suite::shard", "", "When shard is configured as concurrency, saucectl automatically splits the tests by concurrency so that they can easily run in parallel. Requires --name to be set.")
	sc.String("testListFile", "suite::testListFile", "", "This file containing tests will be used in sharding by concurrency. Requires --name to be set.")

	// Test Options
	sc.StringSlice("testOptions.class", "suite::testOptions::class", []string{}, "Only run the specified classes. Requires --name to be set.")
	sc.StringSlice("testOptions.notClass", "suite::testOptions::notClass", []string{}, "Run all classes except those specified here. Requires --name to be set.")

	// Devices
	cmd.Flags().Var(&lflags.Device, "device", "Specifies the device to use for testing. Requires --name to be set.")
	cmd.Flags().Var(&lflags.Simulator, "simulator", "Specifies the simulator to use for testing. Requires --name to be set.")

	// Overwrite devices settings
	sc.Bool("audioCapture", "suite::appSettings::audioCapture", false, "Overwrite app settings for real device to capture audio.")
	sc.Bool("networkCapture", "suite::appSettings::instrumentation::networkCapture", false, "Overwrite app settings for real device to capture network.")

	return cmd
}

func runXcuitest(cmd *cobra.Command, xcuiFlags xcuitestFlags, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

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
	if err := xcuitest.ShardSuites(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := segment.DefaultTracker
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		props := usage.Properties{}
		props.SetFramework("xcuitest").SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).SetArtifacts(p.Artifacts).
			SetNumSuites(len(p.Suites)).SetSlack(p.Notifications.Slack).
			SetSharding(xcuitest.GetShardTypes(p.Suites), nil).SetLaunchOrder(p.Sauce.LaunchOrder).
			SetSmartRetry(p.IsSmartRetried()).SetReporters(p.Reporters)
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	return runXcuitestInCloud(p, regio)
}

func runXcuitestInCloud(p xcuitest.Project, regio region.Region) (int, error) {
	log.Info().Msg("Running XCUITest in Sauce Labs")

	creds := regio.Credentials()

	restoClient := http.NewResto(regio.APIBaseURL(), creds.Username, creds.AccessKey, 0)
	testcompClient := http.NewTestComposer(regio.APIBaseURL(), creds, testComposerTimeout)
	webdriverClient := http.NewWebdriver(regio.WebDriverBaseURL(), creds, webdriverTimeout)
	appsClient := *http.NewAppStore(regio.APIBaseURL(), creds.Username, creds.AccessKey, gFlags.appStoreTimeout)
	rdcClient := http.NewRDCService(regio.APIBaseURL(), creds.Username, creds.AccessKey, rdcTimeout)
	insightsClient := http.NewInsightsService(regio.APIBaseURL(), creds, insightsTimeout)
	iamClient := http.NewUserService(regio.APIBaseURL(), creds, iamTimeout)

	vdcDownloader := downloader.NewArtifactDownloader(&restoClient, p.Artifacts.Download)
	rdcDownloader := downloader.NewArtifactDownloader(&rdcClient, p.Artifacts.Download)
	r := saucecloud.XcuitestRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: &appsClient,
			JobService: saucecloud.JobService{
				VDCStarter:    &webdriverClient,
				RDCStarter:    &rdcClient,
				VDCReader:     &restoClient,
				RDCReader:     &rdcClient,
				VDCWriter:     &testcompClient,
				VDCStopper:    &restoClient,
				RDCStopper:    &rdcClient,
				VDCDownloader: &vdcDownloader,
				RDCDownloader: &rdcDownloader,
			},
			TunnelService:   &restoClient,
			MetadataService: &testcompClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			BuildService:    &restoClient,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
			Reporters: createReporters(
				p.Reporters, p.Notifications, p.Sauce.Metadata, &testcompClient,
				"xcuitest", "sauce", gFlags.async,
			),
			Framework: framework.Framework{Name: xcuitest.Kind},
			Async:     gFlags.async,
			FailFast:  gFlags.failFast,
			Retrier: &retry.JunitRetrier{
				VDCReader: &restoClient,
				RDCReader: &rdcClient,
			},
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
			flags.Device.Changed ||
			flags.Simulator.Changed

		if isErr {
			return ErrEmptySuiteName
		}

		return nil
	}

	if flags.Device.Changed {
		p.Suite.Devices = append(p.Suite.Devices, flags.Device.Device)
	}

	if flags.Simulator.Changed {
		p.Suite.Simulators = append(p.Suite.Simulators, flags.Simulator.Simulator)
	}

	p.Suites = []xcuitest.Suite{p.Suite}

	return nil
}
