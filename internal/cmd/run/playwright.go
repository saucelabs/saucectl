package run

import (
	"errors"
	"fmt"
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
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/saucecloud/retry"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
)

type playwrightFlags struct {
	npmStrictSSL bool
}

// NewPlaywrightCmd creates the 'run' command for Playwright.
func NewPlaywrightCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
	var pf playwrightFlags

	cmd := &cobra.Command{
		Use:              "playwright",
		Short:            "Run playwright tests",
		SilenceUsage:     true,
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::testMatch", args)

			exitCode, err := runPlaywright(cmd, pf, true)
			if err != nil {
				log.Err(err).Msg("failed to execute run command")
			}
			os.Exit(exitCode)
		},
	}

	sc.Fset = cmd.Flags()

	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite::params::browserName", "", "Run tests against this browser")
	sc.String("platform", "suite::platformName", "", "Run tests against this platform")

	// Playwright
	sc.String("playwright.version", "playwright::version", "", "The Playwright version to use")
	sc.String("playwright.configFile", "playwright::configFile", "", "The path to playwright config file")

	// Playwright Test Options
	sc.Bool("headless", "suite::params::headless", false, "Run tests in headless mode")
	sc.Int("globalTimeout", "suite::params::globalTimeout", 0, "Total timeout for the whole test run in milliseconds")
	sc.Int("testTimeout", "suite::params::timeout", 0, "Maximum timeout in milliseconds for each test")
	sc.String("grep", "suite::params::grep", "", "Only run tests matching this regular expression")
	sc.String("grep-invert", "suite::params::grepInvert", "", "Only run tests not matching this regular expression. ")
	sc.Int("repeatEach", "suite::params::repeatEach", 0, "Run each test N times")
	sc.Int("retries", "suite::params::retries", 0, "The maximum number of retries for flaky tests")
	sc.Int("maxFailures", "suite::params::maxFailures", 0, "Stop after the first N test failures")
	sc.Int("numShards", "suite::numShards", 0, "Split tests across N number of shards")
	sc.String("project", "suite::params::project", "", "Specify playwright project")
	sc.StringSlice("excludedTestFiles", "suite::excludedTestFiles", []string{}, "Exclude test files to skip the tests, using regex")
	sc.Bool("updateSnapshots", "suite::params::updateSnapshots", false, "Whether to update expected snapshots with the actual results produced by the test run.")
	sc.Int("workers", "suite::params::workers", 1, "Set the maximum number of parallel worker processes (Default: 1).")

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("shard", "suite.shard", "", "Controls whether or not (and how) tests are sharded across multiple machines, supported value: spec|concurrency")
	sc.String("timeZone", "suite::timeZone", "", "Specifies timeZone for this test")
	sc.Int("passThreshold", "suite::passThreshold", 1, "The minimum number of successful attempts for a suite to be considered as 'passed'.")
	sc.Bool("shardGrepEnabled", "suite::shardGrepEnabled", false, "When sharding is configured and the suite is configured to filter using a pattern, let saucectl filter tests before executing")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.StringSlice("npm.dependencies", "npm::dependencies", []string{}, "Specify local npm dependencies for saucectl to upload. These dependencies must already be installed in the local node_modules directory.")
	cmd.Flags().BoolVar(&pf.npmStrictSSL, "npm.strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https.")

	// Deprecated flags
	_ = sc.Fset.MarkDeprecated("npm.registry", "please set the npm registries field in the Sauce configuration file")
	return cmd
}

func runPlaywright(cmd *cobra.Command, pf playwrightFlags, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := playwright.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())

	if cmd.Flags().Changed("npm.strictSSL") {
		p.Npm.StrictSSL = &pf.npmStrictSSL
	}

	if err := applyPlaywrightFlags(&p); err != nil {
		return 1, err
	}
	playwright.SetDefaults(&p)

	if err := playwright.Validate(&p); err != nil {
		return 1, err
	}

	if err := playwright.ShardSuites(&p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	if regio == region.USEast4 {
		return 1, errors.New(msg.NoFrameworkSupport)
	}

	if !gFlags.noAutoTagging {
		p.Sauce.Metadata.Tags = append(p.Sauce.Metadata.Tags, ci.GetTags()...)
	}

	tracker := segment.DefaultTracker
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		props := usage.Properties{}
		props.SetFramework("playwright").SetFVersion(p.Playwright.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).
			SetSlack(p.Notifications.Slack).SetSharding(playwright.GetShardTypes(p.Suites), playwright.GetShardOpts(p.Suites)).SetLaunchOrder(p.Sauce.LaunchOrder).
			SetSmartRetry(p.IsSmartRetried()).SetReporters(p.Reporters).SetNodeVersion(p.NodeVersion)
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	creds := regio.Credentials()

	restoClient := http.NewResto(regio.APIBaseURL(), creds.Username, creds.AccessKey, 0)
	testcompClient := http.NewTestComposer(regio.APIBaseURL(), creds, testComposerTimeout)
	webdriverClient := http.NewWebdriver(regio.WebDriverBaseURL(), creds, webdriverTimeout)
	appsClient := *http.NewAppStore(regio.APIBaseURL(), creds.Username, creds.AccessKey, gFlags.appStoreTimeout)
	rdcClient := http.NewRDCService(regio.APIBaseURL(), creds.Username, creds.AccessKey, rdcTimeout)
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

	log.Info().Msg("Running Playwright in Sauce Labs")
	r := saucecloud.PlaywrightRunner{
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
			Reporters: createReporters(
				p.Reporters, p.Notifications, p.Sauce.Metadata, &testcompClient,
				"playwright", "sauce", gFlags.async,
			),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.Playwright.Version, p.RootDir),
			NPMDependencies:        p.Npm.Dependencies,
			Retrier: &retry.SauceReportRetrier{
				JobService:      jobService,
				ProjectUploader: &appsClient,
				Project:         &p,
			},
		},
	}

	p.Npm.Packages = cleanPlaywrightPackages(p.Npm, p.Playwright.Version)
	return r.RunProject()
}

func applyPlaywrightFlags(p *playwright.Project) error {
	if gFlags.selectedSuite != "" {
		if err := playwright.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}

	// Use the adhoc suite instead, if one is provided
	if p.Suite.Name != "" {
		p.Suites = []playwright.Suite{p.Suite}
	}

	return nil
}

func cleanPlaywrightPackages(n config.Npm, version string) map[string]string {
	// Don't allow framework installation, it is provided by the runner
	ignoredPackages := []string{}
	playwrightVersion, hasPlaywright := n.Packages["playwright"]
	playwrightTestVersion, hasPlaywrightTest := n.Packages["@playwright/test"]
	if hasPlaywright {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("playwright@%s", playwrightVersion))
	}
	if hasPlaywrightTest {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("@playwright/test@%s", playwrightTestVersion))
	}
	if hasPlaywright || hasPlaywrightTest {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("playwright", version, ignoredPackages))
		return config.CleanNpmPackages(n.Packages, []string{"playwright", "@playwright/test"})
	}
	return n.Packages
}
