package run

import (
	"context"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/ci"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/xctest"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type xctestFlags struct {
	Device flags.Device
}

// NewXCTestCmd creates the 'run' command for XCTest.
func NewXCTestCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
	lflags := xctestFlags{}

	cmd := &cobra.Command{
		Use:              "xctest",
		Short:            "Run xctest tests.",
		Long:             "Unlike 'saucectl run', this command allows you to bypass the config file partially or entirely by configuring an adhoc suite (--name) via flags.",
		Example:          `saucectl run xctest -c "" --name "My Suite" --app app.ipa --xcTestRunFile xcTestRunFile.xctestrun --otherApps=a.ipa,b.ipa --device name="iPhone.*",platformVersion=14.0,carrierConnectivity=false,deviceType=PHONE,private=false`,
		SilenceUsage:     true,
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, _ []string) {
			exitCode, err := runXctest(cmd, lflags, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()
	sc.String("name", "suite::name", "", "Creates a new adhoc suite with this name. Suites defined in the config will be ignored.")
	sc.String("app", "xctest::app", "", "Specifies the app under test")
	sc.String("appDescription", "xctest::appDescription", "", "Specifies description for the app")
	sc.String("xcTestRunFile", "xctest::xcTestRunFile", "", "Specifies the xctestrun test config")
	sc.StringSlice("otherApps", "xctest::otherApps", []string{}, "Specifies any additional apps that are installed alongside the main app")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

	sc.String("shard", "suite::shard", "", "When shard is configured as concurrency, saucectl automatically splits the tests by concurrency so that they can easily run in parallel. Requires --name to be set.")
	sc.String("testListFile", "suite::testListFile", "", "This file containing tests will be used in sharding by concurrency. Requires --name to be set.")

	// Test Options
	sc.StringSlice("testOptions.class", "suite::testOptions::class", []string{}, "Only run the specified classes. Requires --name to be set.")
	sc.StringSlice("testOptions.notClass", "suite::testOptions::notClass", []string{}, "Run all classes except those specified here. Requires --name to be set.")

	// Devices
	cmd.Flags().Var(&lflags.Device, "device", "Specifies the device to use for testing. Requires --name to be set.")

	// Overwrite devices settings
	sc.Bool("resigningEnabled", "suite::appSettings::resigningEnabled", true, "Overwrite app settings for real device to enable app resigning.")
	sc.Bool("audioCapture", "suite::appSettings::audioCapture", false, "Overwrite app settings for real device to capture audio.")
	sc.Bool("imageInjection", "suite::appSettings::instrumentation::imageInjection", false, "Overwrite app settings for real device to inject provided images in the user app.")
	sc.Bool("sysAlertsDelay", "suite::appSettings::instrumentation::sysAlertsDelay", false, "Overwrite app settings for real device to delay system alerts.")
	sc.Bool("vitals", "suite::appSettings::instrumentation::vitals", false, "Overwrite app settings for real device to enable vitals.")
	sc.Bool("networkCapture", "suite::appSettings::instrumentation::networkCapture", false, "Overwrite app settings for real device to capture network.")
	sc.Bool("biometrics", "suite::appSettings::instrumentation::biometrics", false, "Overwrite app settings for real device to intercept biometric authentication.")
	sc.Bool("groupDirectory", "suite::appSettings::instrumentation::groupDirectory", false, "Overwrite app settings for real device to enable group directory access.")

	return cmd
}

func runXctest(cmd *cobra.Command, xcuiFlags xctestFlags, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := xctest.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}
	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())

	if err := applyXCTestFlags(&p, xcuiFlags); err != nil {
		return 1, err
	}
	xctest.SetDefaults(&p)

	if err := xctest.Validate(p); err != nil {
		return 1, err
	}
	if err := xctest.ShardSuites(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := usage.DefaultClient
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Framework("xctest", ""),
			usage.Flags(cmd.Flags()),
			usage.SauceConfig(p.Sauce),
			usage.Artifacts(p.Artifacts),
			usage.NumSuites(len(p.Suites)),
			usage.Sharding(xctest.GetShardTypes(p.Suites), nil),
			usage.SmartRetry(p.IsSmartRetried()),
			usage.Reporters(p.Reporters),
		)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	return runXctestInCloud(cmd.Context(), p, regio)
}

func runXctestInCloud(ctx context.Context, p xctest.Project, regio region.Region) (int, error) {
	log.Info().
		Str("region", regio.String()).
		Str("tunnel", p.Sauce.Tunnel.Name).
		Msg("Running XCTest in Sauce Labs.")

	creds := regio.Credentials()

	restoClient := http.NewResto(regio, creds.Username, creds.AccessKey, 0)
	testcompClient := http.NewTestComposer(regio.APIBaseURL(), creds, testComposerTimeout)
	webdriverClient := http.NewWebdriver(regio, creds, webdriverTimeout)
	appsClient := *http.NewAppStore(regio.APIBaseURL(), creds.Username, creds.AccessKey, gFlags.appStoreTimeout)
	rdcClient := http.NewRDCService(regio, creds.Username, creds.AccessKey, rdcTimeout)
	insightsClient := http.NewInsightsService(regio.APIBaseURL(), creds, insightsTimeout)
	iamClient := http.NewUserService(regio.APIBaseURL(), creds, iamTimeout)
	jobService := saucecloud.JobService{
		RDC:                    rdcClient,
		Resto:                  restoClient,
		Webdriver:              webdriverClient,
		TestComposer:           testcompClient,
		ArtifactDownloadConfig: p.Artifacts.Download,
	}
	buildService := http.NewBuildService(
		regio, creds.Username, creds.AccessKey, buildTimeout,
	)

	r := saucecloud.XctestRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: &appsClient,
			JobService:      jobService,
			TunnelService:   &restoClient,
			MetadataService: &testcompClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			BuildService:    &buildService,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
			Reporters:       createReporters(p.Reporters, gFlags.async),
			Framework:       framework.Framework{Name: xcuitest.Kind},
			Async:           gFlags.async,
			FailFast:        gFlags.failFast,
			Retrier: &retry.JunitRetrier{
				JobService: jobService,
			},
		},
	}
	return r.RunProject(ctx)
}

func applyXCTestFlags(p *xctest.Project, flags xctestFlags) error {
	if gFlags.selectedSuite != "" {
		if err := xctest.FilterSuites(p, gFlags.selectedSuite); err != nil {
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

	p.Suites = []xctest.Suite{p.Suite}

	return nil
}
