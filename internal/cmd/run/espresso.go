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
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/segment"
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
		SilenceUsage:     true,
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, _ []string) {
			exitCode, err := runEspresso(cmd, lflags, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()
	sc.String("name", "suite::name", "", "Sets the name of the job as it will appear on Sauce Labs")
	sc.String("app", "espresso::app", "", "Specifies the app under test")
	sc.String("appDescription", "espresso::appDescription", "", "Specifies description for the app")
	sc.String("testApp", "espresso::testApp", "", "Specifies the test app")
	sc.String("testAppDescription", "espresso::testAppDescription", "", "Specifies description for the testApp")
	sc.StringSlice("otherApps", "espresso::otherApps", []string{}, "Specifies any additional apps that are installed alongside the main app")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

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

	// Overwrite devices settings
	sc.Bool("audioCapture", "suite::appSettings::audioCapture", false, "Overwrite app settings for real device to capture audio.")
	sc.Bool("networkCapture", "suite::appSettings::instrumentation::networkCapture", false, "Overwrite app settings for real device to capture network.")

	return cmd
}

func runEspresso(cmd *cobra.Command, espressoFlags espressoFlags, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := espresso.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())

	if err := applyEspressoFlags(&p, espressoFlags); err != nil {
		return 1, err
	}
	espresso.SetDefaults(&p)

	if err := espresso.Validate(p); err != nil {
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
		props.SetFramework("espresso").SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).SetArtifacts(p.Artifacts).
			SetNumSuites(len(p.Suites)).SetSlack(p.Notifications.Slack).
			SetSharding(espresso.GetShardTypes(p.Suites), nil).SetLaunchOrder(p.Sauce.LaunchOrder).
			SetSmartRetry(p.IsSmartRetried()).SetReporters(p.Reporters)
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	return runEspressoInCloud(p, regio)
}

func runEspressoInCloud(p espresso.Project, regio region.Region) (int, error) {
	log.Info().Msg("Running Espresso in Sauce Labs")

	creds := regio.Credentials()
	restoClient := http.NewResto(regio.APIBaseURL(), creds.Username, creds.AccessKey, 0)
	testcompClient := http.NewTestComposer(regio.APIBaseURL(), creds, testComposerTimeout)
	webdriverClient := http.NewWebdriver(regio.WebDriverBaseURL(), creds, webdriverTimeout)
	appsClient := *http.NewAppStore(regio.APIBaseURL(), creds.Username, creds.AccessKey, gFlags.appStoreTimeout)
	rdcClient := http.NewRDCService(regio.APIBaseURL(), creds.Username, creds.AccessKey, rdcTimeout)
	insightsClient := http.NewInsightsService(regio.APIBaseURL(), creds, insightsTimeout)
	iamClient := http.NewUserService(regio.APIBaseURL(), creds, iamTimeout)

	r := saucecloud.EspressoRunner{
		Project: p,
		CloudRunner: saucecloud.CloudRunner{
			ProjectUploader: &appsClient,
			JobService: saucecloud.JobService{
				RDC:                    rdcClient,
				Resto:                  restoClient,
				Webdriver:              webdriverClient,
				TestComposer:           testcompClient,
				ArtifactDownloadConfig: p.Artifacts.Download,
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
				"espresso", "sauce", gFlags.async,
			),
			Framework: framework.Framework{Name: espresso.Kind},
			Async:     gFlags.async,
			FailFast:  gFlags.failFast,
			Retrier: &retry.JunitRetrier{
				RDCReader: &rdcClient,
				VDCReader: &restoClient,
			},
		},
	}

	return r.RunProject()
}

func hasKey(testOptions map[string]interface{}, key string) bool {
	_, ok := testOptions[key]
	return ok
}

func applyEspressoFlags(p *espresso.Project, flags espressoFlags) error {
	if gFlags.selectedSuite != "" {
		if err := espresso.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	if p.Suite.Name == "" {
		isErr := hasKey(p.Suite.TestOptions, "class") ||
			hasKey(p.Suite.TestOptions, "notClass") ||
			hasKey(p.Suite.TestOptions, "package") ||
			hasKey(p.Suite.TestOptions, "notPackage") ||
			hasKey(p.Suite.TestOptions, "size") ||
			hasKey(p.Suite.TestOptions, "annotation") ||
			hasKey(p.Suite.TestOptions, "notAnnotation") ||
			hasKey(p.Suite.TestOptions, "numShards") ||
			hasKey(p.Suite.TestOptions, "useTestOrchestrator") ||
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
