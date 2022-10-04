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

	"github.com/saucelabs/saucectl/internal/backtrace"
	"github.com/saucelabs/saucectl/internal/ci"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/docker"
	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report/captor"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/saucelabs/saucectl/internal/viper"
)

// NewPlaywrightCmd creates the 'run' command for Playwright.
func NewPlaywrightCmd() *cobra.Command {
	sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}

	cmd := &cobra.Command{
		Use:              "playwright",
		Short:            "Run playwright tests",
		Hidden:           true, // TODO reveal command once ready
		TraverseChildren: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sc.BindAll()
			return preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Test patterns are passed in via positional args.
			viper.Set("suite::testMatch", args)

			exitCode, err := runPlaywright(cmd)
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

	sc.String("name", "suite::name", "", "Set the name of the job as it will appear on Sauce Labs")

	// Browser & Platform
	sc.String("browser", "suite::params::browserName", "", "Run tests against this browser")
	sc.String("platform", "suite::platformName", "", "Run tests against this platform")

	// Playwright
	sc.String("playwright.version", "playwright::version", "", "The Playwright version to use")
	sc.String("playwright.configFile", "playwright::configFile", "", "The path to playwright config file")

	// Playwright Test Options
	sc.Bool("headed", "suite::params::headed", false, "Run tests in headed browsers")
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

	// Misc
	sc.String("rootDir", "rootDir", ".", "Control what files are available in the context of a test run, unless explicitly excluded by .sauceignore")
	sc.String("shard", "suite.shard", "", "Controls whether or not (and how) tests are sharded across multiple machines, supported value: spec|concurrency")
	sc.String("timeZone", "suite::timeZone", "", "Specifies timeZone for this test")

	// NPM
	sc.String("npm.registry", "npm::registry", "", "Specify the npm registry URL")
	sc.StringToString("npm.packages", "npm::packages", map[string]string{}, "Specify npm packages that are required to run tests")
	sc.StringSlice("npm.dependencies", "npm::dependencies", []string{}, "Specify local npm dependencies for saucectl to upload. These dependencies must already be installed in the local node_modules directory.")
	sc.Bool("npm.strictSSL", "npm::strictSSL", true, "Whether or not to do SSL key validation when making requests to the registry via https")

	// Deprecated flags
	_ = sc.Fset.MarkDeprecated("headed", "please use --headless instead")

	return cmd
}

func runPlaywright(cmd *cobra.Command) (int, error) {
	p, err := playwright.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	p.CLIFlags = flags.CaptureCommandLineFlags(cmd.Flags())

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

	go func() {
		props := usage.Properties{}
		props.SetFramework("playwright").SetFVersion(p.Playwright.Version).SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetDocker(p.Docker).SetNPM(p.Npm).SetNumSuites(len(p.Suites)).SetJobs(captor.Default.TestResults).
			SetSlack(p.Notifications.Slack).SetSharding(playwright.IsSharded(p.Suites)).SetLaunchOrder(p.Sauce.LaunchOrder)
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	dockerProject, sauceProject := playwright.SplitSuites(p)
	if len(dockerProject.Suites) != 0 {
		exitCode, err := runPlaywrightInDocker(dockerProject)
		if err != nil || exitCode != 0 {
			return exitCode, err
		}
	}
	if len(sauceProject.Suites) != 0 {
		return runPlaywrightInSauce(sauceProject, regio)
	}

	return 0, nil
}

func runPlaywrightInDocker(p playwright.Project) (int, error) {
	log.Info().Msg("Running Playwright in Docker")
	printTestEnv("docker")

	cd, err := docker.NewPlaywright(p, &testcompClient, &testcompClient, &restoClient, &restoClient, createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &testcompClient, &restoClient,
		"playwright", "docker"))
	if err != nil {
		return 1, err
	}

	cleanPlaywrightPackages(&p)
	return cd.RunProject()
}

func runPlaywrightInSauce(p playwright.Project, regio region.Region) (int, error) {
	log.Info().Msg("Running Playwright in Sauce Labs")
	printTestEnv("sauce")

	r := saucecloud.PlaywrightRunner{
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
			CCYReader:       &restoClient,
			TunnelService:   &restoClient,
			MetadataService: &testcompClient,
			InsightsService: &insightsClient,
			UserService:     &iamClient,
			Region:          regio,
			ShowConsoleLog:  p.ShowConsoleLog,
			Reporters: createReporters(p.Reporters, p.Notifications, p.Sauce.Metadata, &testcompClient, &restoClient,
				"playwright", "sauce"),
			Async:                  gFlags.async,
			FailFast:               gFlags.failFast,
			MetadataSearchStrategy: framework.NewSearchStrategy(p.Playwright.Version, p.RootDir),
			NPMDependencies:        p.Npm.Dependencies,
		},
	}

	cleanPlaywrightPackages(&p)
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

func cleanPlaywrightPackages(p *playwright.Project) {
	// Don't allow framework installation, it is provided by the runner
	ignoredPackages := []string{}
	playwrightVersion, hasPlaywright := p.Npm.Packages["playwright"]
	playwrightTestVersion, hasPlaywrightTest := p.Npm.Packages["@playwright/test"]
	if hasPlaywright {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("playwright@%s", playwrightVersion))
	}
	if hasPlaywrightTest {
		ignoredPackages = append(ignoredPackages, fmt.Sprintf("@playwright/test@%s", playwrightTestVersion))
	}
	if hasPlaywright || hasPlaywrightTest {
		log.Warn().Msg(msg.IgnoredNpmPackagesMsg("playwright", p.Playwright.Version, ignoredPackages))
		p.Npm.Packages = config.CleanNpmPackages(p.Npm.Packages, []string{"playwright", "@playwright/test"})
	}
}
