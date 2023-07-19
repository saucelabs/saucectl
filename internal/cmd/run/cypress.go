package run

import (
	"errors"
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
)

// NewCypressCmd creates the 'run' command for Cypress.
func NewCypressCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "cypress",
		Short:            "Run cypress tests",
		SilenceUsage:     true,
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::config::specPattern", args)

			exitCode, err := runCypress(cmd, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()
	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite::browser", "", "Run tests against this browser")
	sc.String("browserVersion", "suite::browserVersion", "", "The browser version (default: latest)")
	sc.String("platform", "suite::platformName", "", "Run tests against this platform")

	// Cypress
	sc.String("cypress.version", "cypress::version", "", "The Cypress version to use")
	sc.String("cypress.configFile", "cypress::configFile", "", "The path to the cypress.json config file")
	sc.String("cypress.key", "cypress::key", "", "The Cypress record key")
	sc.Bool("cypress.record", "cypress::record", false, "Whether or not to record tests to the cypress dashboard")
	sc.StringSlice("excludeSpecPattern", "suite::config::excludeSpecPattern", []string{}, "Exclude test files to skip the tests, using glob pattern")
	sc.String("testingType", "suite::config::testingType", "e2e", "Specify the type of tests to execute; either e2e or component. Defaults to e2e")

	// Video & Screen(shots)
	sc.String("screenResolution", "suite::screenResolution", "", "The screen resolution")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("shard", "suite::shard", "", "Controls whether or not (and how) tests are sharded across multiple machines, supported value: spec|concurrency")
	sc.Bool("shardGrepEnabled", "suite::shardGrepEnabled", false, "When sharding is configured and the suite is configured to filter using cypress-grep, let saucectl filter tests before executing")
	sc.String("headless", "suite::headless", "", "Controls whether or not tests are run in headless mode (default: false)")
	sc.String("timeZone", "suite::timeZone", "", "Specifies timeZone for this test")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.StringSlice("npm.dependencies", "npm::dependencies", []string{}, "Specify local npm dependencies for saucectl to upload. These dependencies must already be installed in the local node_modules directory.")
	sc.Bool("npm.strictSSL", "npm::strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	return cmd
}

func runCypress(cmd *cobra.Command, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := cypress.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.SetCLIFlags(flags.CaptureCommandLineFlags(cmd.Flags()))
	if err := p.ApplyFlags(gFlags.selectedSuite); err != nil {
		return 1, err
	}
	p.SetDefaults()
	if !gFlags.noAutoTagging {
		p.AppendTags(ci.GetTags())
	}

	if err := p.Validate(); err != nil {
		return 1, err
	}

	regio := region.FromString(p.GetSauceCfg().Region)
	if regio == region.USEast4 {
		return 1, errors.New(msg.NoFrameworkSupport)
	}

	testcompClient.URL = regio.APIBaseURL()
	webdriverClient.URL = regio.WebDriverBaseURL()
	restoClient.URL = regio.APIBaseURL()
	appsClient.URL = regio.APIBaseURL()
	insightsClient.URL = regio.APIBaseURL()
	iamClient.URL = regio.APIBaseURL()
	restoClient.ArtifactConfig = p.GetArtifactsCfg().Download
	tracker := segment.DefaultTracker
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		props := usage.Properties{}
		props.SetFramework("cypress").SetFVersion(p.GetVersion()).SetFlags(cmd.Flags()).SetSauceConfig(p.GetSauceCfg()).
			SetArtifacts(p.GetArtifactsCfg()).SetNPM(p.GetNpm()).SetNumSuites(len(p.GetSuites())).SetJobs(captor.Default.TestResults).
			SetSlack(p.GetNotifications().Slack).SetSharding(p.IsSharded()).SetLaunchOrder(p.GetSauceCfg().LaunchOrder).
			SetSmartRetry(p.IsSmartRetried())

		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.GetArtifactsCfg())

	log.Info().Msg("Running Cypress in Sauce Labs")
	r := saucecloud.CypressRunner{
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
			MetadataService: &testcompClient,
			TunnelService:   &restoClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			BuildService:    &restoClient,
			Region:          regio,
			ShowConsoleLog:  p.IsShowConsoleLog(),
			Reporters: createReporters(p.GetReporter(), p.GetNotifications(), p.GetSauceCfg().Metadata, &testcompClient, &restoClient,
				"cypress", "sauce", gFlags.async),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.GetVersion(), p.GetRootDir()),
			NPMDependencies:        p.GetNpm().Dependencies,
			Retrier: &retry.SauceReportRetrier{
				VDCReader:       &restoClient,
				ProjectUploader: &appsClient,
				Project:         p,
			},
		},
	}

	p.CleanPackages()
	return r.RunProject()
}
