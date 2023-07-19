package run

import (
	"errors"
	"fmt"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
)

type testcafeFlags struct {
	QuarantineMode flags.QuarantineMode
	Simulator      flags.Simulator
}

// NewTestcafeCmd creates the 'run' command for TestCafe.
func NewTestcafeCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
	lflags := testcafeFlags{}

	cmd := &cobra.Command{
		Use:              "testcafe",
		Short:            "Run testcafe tests",
		SilenceUsage:     true,
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::src", args)

			exitCode, err := runTestcafe(cmd, lflags, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	f := cmd.Flags()
	sc.Fset = cmd.Flags()
	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite::browserName", "", "Run tests against this browser")
	sc.String("browserVersion", "suite::browserVersion", "", "The browser version (default: latest)")
	sc.StringSlice("browserArgs", "suite::browserArgs", []string{}, "Set browser args")
	sc.String("platform", "suite::platformName", "", "Run tests against this platform")
	sc.Bool("headless", "suite::headless", false, "Controls whether or not tests are run in headless mode (default: false)")

	// Video & Screen(shots)
	sc.Bool("disableScreenshots", "suite::disableScreenshots", false, "Prevent TestCafe from taking screenshots")
	sc.String("screenResolution", "suite::screenResolution", "", "The screen resolution")
	sc.Bool("screenshots.takeOnFails", "suite::screenshots::takeOnFails", false, "Take screenshot on test failure")
	sc.Bool("screenshots.fullPage", "suite::screenshots::fullPage", false, "Take screenshots of the entire page")

	// Error Handling
	f.Var(&lflags.QuarantineMode, "quarantineMode", "Enable quarantine mode to eliminate false negatives and detect unstable tests")
	sc.Bool("skipJsErrors", "suite::skipJsErrors", false, "Ignore JavaScript errors that occur on a tested web page")
	sc.Bool("skipUncaughtErrors", "suite::skipUncaughtErrors", false, "Ignore uncaught errors or unhandled promise rejections on the server during test execution")
	sc.Bool("stopOnFirstFail", "suite::stopOnFirstFail", false, "Stop an entire test run if any test fails")

	// Timeouts
	sc.Int("selectorTimeout", "suite::selectorTimeout", 10000, "Specify the time (in milliseconds) within which selectors attempt to return a node")
	sc.Int("assertionTimeout", "suite::assertionTimeout", 3000, "Specify the time (in milliseconds) TestCafe attempts to successfully execute an assertion")
	sc.Int("pageLoadTimeout", "suite::pageLoadTimeout", 3000, "Specify the time (in milliseconds) passed after the DOMContentLoaded event, within which TestCafe waits for the window.load event to fire")
	sc.Int("ajaxRequestTimeout", "suite::ajaxRequestTimeout", 120000, "Specifies wait time (in milliseconds) for fetch/XHR requests")
	sc.Int("pageRequestTimeout", "suite::pageRequestTimeout", 25000, "Specifies time (in milliseconds) to wait for HTML pages")
	sc.Int("browserInitTimeout", "suite::browserInitTimeout", 120000, "Time (in milliseconds) for browsers to connect to TestCafe and report that they are ready to test")
	sc.Int("testExecutionTimeout", "suite::testExecutionTimeout", 180000, "Maximum test execution time (in milliseconds)")
	sc.Int("runExecutionTimeout", "suite::runExecutionTimeout", 1800000, "Maximum test run execution time (in milliseconds)")

	// Filters
	sc.String("filter.test", "suite::filter::test", "", "Runs a test with the specified name")
	sc.String("filter.testGrep", "suite::filter::testGrep", "", "Runs tests whose names match the specified grep pattern")
	sc.String("filter.fixture", "suite::filter::fixture", "", "Runs a test with the specified fixture name")
	sc.String("filter.fixtureGrep", "suite::filter::fixtureGrep", "", "Runs tests whose fixture names match the specified grep pattern")
	sc.StringToString("filter.testMeta", "suite::filter::testMeta", map[string]string{}, "Runs tests whose metadata matches the specified key-value pair")
	sc.StringToString("filter.fixtureMeta", "suite::filter::fixtureMeta", map[string]string{}, "Runs tests whose fixture’s metadata matches the specified key-value pair")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("testcafe.version", "testcafe::version", "", "The TestCafe version to use")
	sc.StringSlice("clientScripts", "suite::clientScripts", []string{}, "Inject scripts from the specified files into each page visited during the tests")
	sc.Float64("speed", "suite::speed", 1, "Specify the test execution speed")
	sc.Bool("disablePageCaching", "suite::disablePageCaching", false, "Prevent the browser from caching page content")
	sc.StringSlice("excludedTestFiles", "suite::excludedTestFiles", []string{}, "Exclude test files to skip the tests, using glob pattern")
	sc.String("timeZone", "suite::timeZone", "", "Specifies timeZone for this test")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.StringSlice("npm.dependencies", "npm::dependencies", []string{}, "Specify local npm dependencies for saucectl to upload. These dependencies must already be installed in the local node_modules directory.")
	sc.Bool("npm.strictSSL", "npm::strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	// Simulators
	f.Var(&lflags.Simulator, "simulator", "Specifies the simulator to use for testing")

	return cmd
}

func runTestcafe(cmd *cobra.Command, tcFlags testcafeFlags, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := testcafe.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())

	if err := applyTestcafeFlags(&p, tcFlags); err != nil {
		return 1, err
	}
	testcafe.SetDefaults(&p)

	if err := testcafe.Validate(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.USEast4 {
		return 1, errors.New(msg.NoFrameworkSupport)
	}

	webdriverClient.URL = regio.WebDriverBaseURL()
	testcompClient.URL = regio.APIBaseURL()
	restoClient.URL = regio.APIBaseURL()
	appsClient.URL = regio.APIBaseURL()
	insightsClient.URL = regio.APIBaseURL()
	iamClient.URL = regio.APIBaseURL()

	restoClient.ArtifactConfig = p.Artifacts.Download

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := segment.DefaultTracker
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		props := usage.Properties{}
		props.SetFramework("testcafe").SetFVersion(p.Testcafe.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack).SetSharding(testcafe.IsSharded(p.Suites)).SetLaunchOrder(p.Sauce.LaunchOrder).
			SetSmartRetry(p.IsSmartRetried())
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	log.Info().Msg("Running Testcafe in Sauce Labs")
	r := saucecloud.TestcafeRunner{
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
				VDCDownloader: &restoClient,
			},
			TunnelService:   &restoClient,
			MetadataService: &testcompClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			BuildService:    &restoClient,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &testcompClient, &restoClient,
				"testcafe", "sauce", gFlags.async),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.Testcafe.Version, p.RootDir),
			NPMDependencies:        p.Npm.Dependencies,
			Retrier: &retry.SauceReportRetrier{
				VDCReader:       &restoClient,
				ProjectUploader: &appsClient,
				Project:         &p,
			},
		},
	}

	cleanTestCafePackages(&p)
	return r.RunProject()
}

func applyTestcafeFlags(p *testcafe.Project, flags testcafeFlags) error {
	if gFlags.selectedSuite != "" {
		if err := testcafe.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	if p.Suite.Name == "" {
		return nil
	}

	if flags.QuarantineMode.Changed {
		p.Suite.QuarantineMode = flags.QuarantineMode.Values
	}

	if flags.Simulator.Changed {
		p.Suite.Simulators = []config.Simulator{flags.Simulator.Simulator}
	}

	p.Suites = []testcafe.Suite{p.Suite}

	return nil
}

func cleanTestCafePackages(p *testcafe.Project) {
	version, hasFramework := p.Npm.Packages["testcafe"]
	if hasFramework {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("testcafe", p.Testcafe.Version, []string{fmt.Sprintf("testcafe@%s", version)}))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"testcafe"})
	}
}
